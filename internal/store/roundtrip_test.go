package store

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Repository round-trip coverage for the full tenancy data model — MPE-31.
// Each test runs against real Postgres via testPool.

func mkClient(t *testing.T, q *Queries) string {
	t.Helper()
	id := testID("cli")
	if _, err := q.CreateClient(context.Background(),
		CreateClientParams{ID: id, Name: id, Status: "active"}); err != nil {
		t.Fatalf("client %s: %v", id, err)
	}
	return id
}

func mkCustomer(t *testing.T, q *Queries, clientID string) string {
	t.Helper()
	id := testID("cust")
	if _, err := q.CreateCustomer(context.Background(), CreateCustomerParams{
		ID: id, ClientID: clientID, Name: id,
		ExternalRef: pgtype.Text{String: id + "-ref", Valid: true},
	}); err != nil {
		t.Fatalf("customer %s: %v", id, err)
	}
	return id
}

func mkMasterNamed(t *testing.T, q *Queries, clientID, name string, version int32) string {
	t.Helper()
	id := testID("tm")
	if _, err := q.CreateTemplateMaster(context.Background(), CreateTemplateMasterParams{
		ID: id, ClientID: clientID, Name: name,
		Html: "<p>x</p>", Css: "p{}", Version: version, Defaults: []byte("{}"),
	}); err != nil {
		t.Fatalf("master %s: %v", id, err)
	}
	return id
}

func TestClientUpdateAndStatus(t *testing.T) {
	q := New(testPool(t))
	ctx := context.Background()
	id := mkClient(t, q)

	got, err := q.UpdateClient(ctx, UpdateClientParams{
		ID: id, Name: pgtype.Text{String: "renamed", Valid: true},
	})
	if err != nil || got.Name != "renamed" || got.Status != "active" {
		t.Fatalf("update name: %v %v", got, err)
	}
	got, err = q.SetClientStatus(ctx, SetClientStatusParams{
		ID: id, Status: "suspended",
	})
	if err != nil || got.Status != "suspended" {
		t.Fatalf("set status: %v %v", got, err)
	}
	list, err := q.ListClients(ctx)
	if err != nil || !slices.ContainsFunc(list, func(c Client) bool { return c.ID == id }) {
		t.Fatalf("list: %v %v", len(list), err)
	}
}

func TestCustomerUpdateAndList(t *testing.T) {
	q := New(testPool(t))
	ctx := context.Background()
	cli := mkClient(t, q)
	cust := mkCustomer(t, q, cli)

	got, err := q.UpdateCustomer(ctx, UpdateCustomerParams{
		ID: cust, Name: pgtype.Text{String: "new name", Valid: true},
		WebhookUrl: pgtype.Text{String: "https://hooks.local/x", Valid: true},
	})
	if err != nil || got.Name != "new name" || got.WebhookUrl.String != "https://hooks.local/x" {
		t.Fatalf("update customer: %+v %v", got, err)
	}
	list, err := q.ListCustomersForClient(ctx, cli)
	if err != nil || len(list) != 1 || list[0].ID != cust {
		t.Fatalf("list customers: %v %v", list, err)
	}
	byRef, err := q.GetCustomerByExternalRef(ctx, GetCustomerByExternalRefParams{
		ClientID: cli, ExternalRef: pgtype.Text{String: cust + "-ref", Valid: true},
	})
	if err != nil || byRef.ID != cust {
		t.Fatalf("by external ref: %v %v", byRef.ID, err)
	}
}

func TestAPIKeyLifecycleRoundTrip(t *testing.T) {
	q := New(testPool(t))
	ctx := context.Background()
	cli := mkClient(t, q)

	oldPlain := "plain_" + testID("k")
	key, err := q.CreateAPIKey(ctx, CreateAPIKeyParams{
		ID: testID("key"), ClientID: cli, KeyHash: sha256hex(oldPlain),
		Label: "rot", Scopes: []string{"generate"},
	})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}

	// Rotation replaces the hash — old material stops resolving (the key
	// model's "expired" case; there is no expires_at column by design).
	newPlain := "plain_" + testID("k")
	if _, err := q.RotateAPIKey(ctx, RotateAPIKeyParams{
		ID: key.ID, KeyHash: sha256hex(newPlain),
	}); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if _, err := q.GetActiveAPIKeyByHash(ctx, sha256hex(oldPlain)); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("old hash must not resolve after rotation: %v", err)
	}
	got, err := q.GetActiveAPIKeyByHash(ctx, sha256hex(newPlain))
	if err != nil || !got.RotatedAt.Valid {
		t.Fatalf("new hash resolves with rotated_at: %+v %v", got, err)
	}

	if _, err := q.GetAPIKey(ctx, key.ID); err != nil {
		t.Fatalf("get by id: %v", err)
	}
	list, err := q.ListAPIKeysForClient(ctx, cli)
	if err != nil || len(list) != 1 {
		t.Fatalf("list keys: %v %v", len(list), err)
	}
}

func TestTemplateRepositoryRoundTrip(t *testing.T) {
	q := New(testPool(t))
	ctx := context.Background()
	cli := mkClient(t, q)
	cust := mkCustomer(t, q, cli)
	name := testID("tpl")

	mkMasterNamed(t, q, cli, name, 1)
	v2 := mkMasterNamed(t, q, cli, name, 2)
	// Versions are counted per (client, template name).
	if v, err := q.NextMasterVersion(ctx, NextMasterVersionParams{
		ClientID: cli, Name: name,
	}); err != nil || v != 3 {
		t.Fatalf("next version after v2: %d %v", v, err)
	}

	// Tenant-scoped reads.
	if _, err := q.GetMasterForClient(ctx, GetMasterForClientParams{ID: v2, ClientID: cli}); err != nil {
		t.Fatalf("master for client: %v", err)
	}
	other := mkClient(t, q)
	if _, err := q.GetMasterForClient(ctx, GetMasterForClientParams{ID: v2, ClientID: other}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("cross-tenant master read must fail: %v", err)
	}
	masters, err := q.ListMasters(ctx, cli)
	if err != nil || len(masters) < 2 {
		t.Fatalf("list masters: %v %v", len(masters), err)
	}

	// Variant upsert → update round-trip.
	ov1 := `{"toggles":{"header":false}}`
	vr, err := q.UpsertVariant(ctx, UpsertVariantParams{
		ID: testID("var"), MasterID: v2, CustomerID: cust, Overlay: []byte(ov1),
	})
	if err != nil {
		t.Fatalf("upsert variant: %v", err)
	}
	ov2 := `{"toggles":{"header":true},"css_patch":"h1{color:red}"}`
	vr2, err := q.UpsertVariant(ctx, UpsertVariantParams{
		ID: vr.ID, MasterID: v2, CustomerID: cust, Overlay: []byte(ov2),
	})
	if err != nil {
		t.Fatalf("variant update: %v", err)
	}
	variants, err := q.ListVariantsForCustomer(ctx, cust)
	if err != nil || len(variants) != 1 || variants[0].ID != vr.ID {
		t.Fatalf("list variants: %v %v", variants, err)
	}
	resolved, err := q.ResolveVariant(ctx, ResolveVariantParams{
		ClientID: cli, Name: name, CustomerID: cust,
	})
	if err != nil {
		t.Fatalf("resolve variant: %v", err)
	}
	// JSONB round-trips with canonical spacing — compare semantics, not bytes.
	var want, gotOV map[string]any
	if err := json.Unmarshal([]byte(ov2), &want); err != nil {
		t.Fatal(err)
	}
	for src, ov := range map[string]json.RawMessage{"updated": vr2.Overlay, "resolved": resolved.Overlay} {
		gotOV = nil
		if err := json.Unmarshal(ov, &gotOV); err != nil || !reflect.DeepEqual(want, gotOV) {
			t.Fatalf("%s overlay: %s %v", src, ov, err)
		}
	}
}
