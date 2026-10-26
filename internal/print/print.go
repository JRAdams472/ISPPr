// Package print post-processes rendered PDFs through Ghostscript
// (MPE-56): RGB→CMYK conversion under an output ICC profile, forced font
// embedding, and PDF/X-style constraint stripping — the pipeline a
// gang/rip workflow expects.
package print

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Config steers the Ghostscript invocation; FromEnv builds it.
type Config struct {
	GSPath  string        // gs binary (PRINT_GS_PATH, default "gs")
	ICCPath string        // -sOutputICCProfile (PRINT_ICC_PROFILE; empty = none)
	PDFX    bool          // -dPDFX, only emitted when ICCPath is set
	Timeout time.Duration // per-conversion deadline (PRINT_TIMEOUT_MS)
	Enabled bool          // post-processing available at all
}

// Env returns a Config from PRINT_* environment variables.
func Env(getenv func(string) string) Config {
	cfg := Config{
		GSPath:  or(getenv("PRINT_GS_PATH"), "gs"),
		ICCPath: getenv("PRINT_ICC_PROFILE"),
		PDFX:    getenv("PRINT_PDFX") != "false",
		Timeout: 20 * time.Second,
	}
	if ms := getenv("PRINT_TIMEOUT_MS"); ms != "" {
		if d, err := time.ParseDuration(ms + "ms"); err == nil {
			cfg.Timeout = d
		}
	}
	cfg.Enabled = cfg.GSPath != ""
	return cfg
}

func or(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// pdfxPreamble is the pdfmark prelude registering a PDF/X OutputIntent —
// the pattern from gs's shipped PDFX_def.ps, minimalized. The ICC stream
// reads the profile copied alongside the input (SAFER permits cwd reads).
const pdfxPreamble = `[/_objdef {icc_PDFX} /type /stream /OBJ pdfmark
[{icc_PDFX} << /N 4 >> /PUT pdfmark
[{icc_PDFX} (icc.icc) (r) file /PUT pdfmark
[/_objdef {OutputIntent_PDFX} /type /dict /OBJ pdfmark
[{OutputIntent_PDFX} << /Type /OutputIntent /S /GTS_PDFX
  /OutputConditionIdentifier (CGATS TR001) /Info (U.S. Web Coated SWOP v2)
  /RegistryName (http://www.color.org) /DestOutputProfile {icc_PDFX} >> /PUT pdfmark
[{Catalog} <</OutputIntents [ {OutputIntent_PDFX} ]>> /PUT pdfmark
`

// resolveICC finds a profile by name in the usual gs resource dirs when
// the configured value isn't already a resolvable path.
func resolveICC(path string) string {
	if _, err := os.Stat(path); err == nil {
		abs, _ := filepath.Abs(path)
		return abs
	}
	for _, dir := range []string{
		"/usr/share/ghostscript/iccprofiles",
		"/usr/share/ghostscript/lib",
	} {
		p := filepath.Join(dir, path)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Debian/Ubuntu version the resource dir: /usr/share/ghostscript/<ver>/iccprofiles.
	if matches, _ := filepath.Glob("/usr/share/ghostscript/*/iccprofiles/" + path); len(matches) > 0 {
		return matches[0]
	}
	return ""
}

func (c Config) args(in, out string, inputs ...string) []string {
	a := []string{
		// NOSAFER: the pdfmark preamble must (r) file the ICC profile —
		// SAFER blocks all file opens. Inputs are trusted (our own
		// Gotenberg output + generated preamble).
		"-dBATCH", "-dNOPAUSE", "-dQUIET", "-dNOSAFER",
		"-sDEVICE=pdfwrite",
		"-dPDFSETTINGS=/prepress",
		"-sColorConversionStrategy=CMYK",
		"-dProcessColorModel=/DeviceCMYK",
	}
	if c.PDFX && c.ICCPath != "" {
		a = append(a, "-dPDFX")
	}
	return append(append(a, "-sOutputFile="+out), append(inputs, in)...)
}

// Error wraps a failed gs invocation; post-processing failures are
// permanent (bad input or misconfiguration), never retried.
type Error struct{ Err error }

func (e Error) Error() string { return "print post-processing: " + e.Err.Error() }
func (e Error) Unwrap() error { return e.Err }

// Convert pipes a PDF through gs and returns the post-processed bytes.
// It writes in/out as temp files because gs needs real paths.
func Convert(ctx context.Context, cfg Config, pdf []byte) ([]byte, error) {
	dir, err := os.MkdirTemp("", "isppr-print-")
	if err != nil {
		return nil, Error{Err: err}
	}
	defer os.RemoveAll(dir)

	in := filepath.Join(dir, "in.pdf")
	out := filepath.Join(dir, "out.pdf")
	if err := os.WriteFile(in, pdf, 0o600); err != nil {
		return nil, Error{Err: err}
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	// PDF/X: stage the ICC profile + pdfmark preamble in the temp dir —
	// SAFER permits reads from the working directory.
	var extra []string
	if cfg.PDFX && cfg.ICCPath != "" {
		if icc := resolveICC(cfg.ICCPath); icc != "" {
			if prof, rerr := os.ReadFile(icc); rerr == nil {
				_ = os.WriteFile(filepath.Join(dir, "icc.icc"), prof, 0o600)
				_ = os.WriteFile(filepath.Join(dir, "pre.ps"), []byte(pdfxPreamble), 0o600)
				extra = []string{filepath.Join(dir, "pre.ps")}
			}
		}
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, cfg.GSPath, cfg.args(in, out, extra...)...)
	cmd.Dir = dir
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, Error{
			Err: fmt.Errorf("gs: %w (stderr: %s)", err, stderr.String()),
		}
	}
	res, err := os.ReadFile(out)
	if err != nil {
		return nil, Error{Err: err}
	}
	return res, nil
}
