package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
)

// Presigner is an optional Backend capability: backends that can mint
// temporary download URLs (S3/MinIO) implement it; the download route
// redirects to the URL and response bodies embed it directly. Local
// backends leave it unimplemented and stream bytes instead.
type Presigner interface {
	Presign(ctx context.Context, key string) (string, error)
}

// S3Config holds S3/MinIO connection settings. PublicEndpoint is where
// presigned URLs are minted — it must resolve for the *caller* (e.g.
// http://localhost:9100 in compose), while Endpoint is the in-cluster
// address used for reads/writes.
type S3Config struct {
	Endpoint       string
	PublicEndpoint string
	Bucket         string
	AccessKey      string
	SecretKey      string
	// Region signs presigned URLs without a GetBucketLocation lookup —
	// required because the public endpoint may not resolve in-cluster.
	Region string
	// PresignTTL bounds every download URL (spec: 15 minutes).
	PresignTTL time.Duration
}

// S3Store stores artifacts in an S3/MinIO bucket and presigns downloads.
type S3Store struct {
	client *minio.Client
	signer *minio.Client
	bucket string
	ttl    time.Duration
}

// NewS3 builds the store, ensuring the bucket exists and carrying a
// 1-day lifecycle-expiry backstop (AWS lifecycle can't express sub-day
// TTLs — the presigned URL enforces the real 15-minute contract).
func NewS3(ctx context.Context, cfg S3Config) (*S3Store, error) {
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	client, err := dial(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey, region)
	if err != nil {
		return nil, fmt.Errorf("s3 endpoint: %w", err)
	}
	pub := cfg.PublicEndpoint
	if pub == "" {
		pub = cfg.Endpoint
	}
	signer, err := dial(pub, cfg.AccessKey, cfg.SecretKey, region)
	if err != nil {
		return nil, fmt.Errorf("s3 public endpoint: %w", err)
	}
	s := &S3Store{client: client, signer: signer, bucket: cfg.Bucket, ttl: cfg.PresignTTL}
	if s.ttl <= 0 {
		s.ttl = 15 * time.Minute
	}
	if err := s.ensureBucket(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func dial(endpoint, access, secret, region string) (*minio.Client, error) {
	raw := endpoint
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw // bare host[:port] — url.Parse eats it as scheme
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	return minio.New(u.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(access, secret, ""),
		Secure: u.Scheme == "https",
		Region: region,
	})
}

func (s *S3Store) ensureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("s3 bucket check: %w", err)
	}
	if !exists {
		if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("s3 bucket create: %w", err)
		}
	}
	lc := lifecycle.NewConfiguration()
	lc.Rules = []lifecycle.Rule{{
		ID:         "expire-artifacts",
		Status:     "Enabled",
		Expiration: lifecycle.Expiration{Days: 1},
	}}
	if err := s.client.SetBucketLifecycle(ctx, s.bucket, lc); err != nil {
		return fmt.Errorf("s3 lifecycle: %w", err)
	}
	return nil
}

func (s *S3Store) Put(ctx context.Context, key string, r io.Reader) error {
	key = strings.TrimPrefix(key, "/")
	_, err := s.client.PutObject(ctx, s.bucket, key, r, -1,
		minio.PutObjectOptions{ContentType: "application/octet-stream"})
	return err
}

func (s *S3Store) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	return s.client.GetObject(ctx, s.bucket, strings.TrimPrefix(key, "/"),
		minio.GetObjectOptions{})
}

// Presign mints a GET URL valid for the configured TTL, signed against
// the public endpoint so callers outside the cluster can resolve it.
func (s *S3Store) Presign(ctx context.Context, key string) (string, error) {
	u, err := s.signer.PresignedGetObject(ctx, s.bucket,
		strings.TrimPrefix(key, "/"), s.ttl, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
