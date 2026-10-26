package print

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeGS compiles a stub gs binary: copies input→output, appends a marker.
func fakeGS(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := `package main
import ("os";"strings")
func main(){
  var in,out string
  for _,a := range os.Args[1:] {
    if strings.HasPrefix(a,"-sOutputFile=") { out = a[13:] } else if !strings.HasPrefix(a,"-") { in = a }
  }
  b,_ := os.ReadFile(in)
  os.WriteFile(out, append(b, []byte("%GS")...), 0600)
}`
	sp := filepath.Join(dir, "gs.go")
	exe := filepath.Join(dir, "gs.exe")
	if err := os.WriteFile(sp, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("go", "build", "-o", exe, sp).CombinedOutput(); err != nil {
		t.Fatalf("fake gs build: %v %s", err, out)
	}
	return exe
}

func TestArgsFlags(t *testing.T) {
	cfg := Config{GSPath: "gs", ICCPath: "icc.icc", PDFX: true}
	a := strings.Join(cfg.args("in.pdf", "out.pdf"), " ")
	for _, want := range []string{
		"-sDEVICE=pdfwrite", "-dPDFSETTINGS=/prepress",
		"-sColorConversionStrategy=CMYK", "-dProcessColorModel=/DeviceCMYK",
		"-dPDFX", "-sOutputFile=out.pdf", "in.pdf",
	} {
		if !strings.Contains(a, want) {
			t.Errorf("args missing %q: %s", want, a)
		}
	}
}

func TestArgsNoICCNoPDFX(t *testing.T) {
	// No ICC profile → no PDFX flag (the OutputIntent needs a profile).
	cfg := Config{GSPath: "gs", PDFX: true}
	a := strings.Join(cfg.args("i", "o"), " ")
	if strings.Contains(a, "-dPDFX") {
		t.Errorf("PDFX emitted without ICC path: %s", a)
	}
}

func TestConvertPipesThroughGS(t *testing.T) {
	cfg := Config{
		GSPath: fakeGS(t), ICCPath: "default_cmyk.icc",
		PDFX: true, Timeout: 30 * time.Second, Enabled: true,
	}
	out, err := Convert(context.Background(), cfg, []byte("%PDF-fake"))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if string(out) != "%PDF-fake%GS" {
		t.Fatalf("post-processed bytes wrong: %q", out)
	}
}

func TestConvertMissingGS(t *testing.T) {
	cfg := Config{GSPath: "definitely-not-gs-9x", Timeout: time.Second}
	_, err := Convert(context.Background(), cfg, []byte("%PDF"))
	if err == nil {
		t.Fatal("want error for missing gs")
	}
	var pe Error
	if !errors.As(err, &pe) || !strings.Contains(err.Error(), "print post-processing") {
		t.Fatalf("want print.Error, got %v", err)
	}
}

func TestEnvDefaults(t *testing.T) {
	cfg := Env(func(k string) string { return map[string]string{}[k] })
	if cfg.GSPath != "gs" || cfg.PDFX != true || cfg.Timeout != 20*time.Second || !cfg.Enabled {
		t.Errorf("bad defaults: %+v", cfg)
	}
	cfg = Env(func(k string) string {
		return map[string]string{
			"PRINT_GS_PATH": "gs10", "PRINT_ICC_PROFILE": "swop.icc",
			"PRINT_PDFX": "false", "PRINT_TIMEOUT_MS": "5000",
		}[k]
	})
	if cfg.GSPath != "gs10" || cfg.ICCPath != "swop.icc" || cfg.PDFX || cfg.Timeout != 5*time.Second {
		t.Errorf("bad env parse: %+v", cfg)
	}
}

func TestResolveICC(t *testing.T) {
	// Existing file resolves absolute.
	f := filepath.Join(t.TempDir(), "p.icc")
	if err := os.WriteFile(f, []byte("icc"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := resolveICC(f); got == "" {
		t.Fatal("existing profile not resolved")
	}
	if got := resolveICC("definitely-missing-9x.icc"); got != "" {
		t.Fatalf("missing profile resolved: %s", got)
	}
}

func TestConvertWithICCStagesPreamble(t *testing.T) {
	// Real profile file → icc.icc + pre.ps are staged into the gs temp dir.
	icc := filepath.Join(t.TempDir(), "p.icc")
	if err := os.WriteFile(icc, []byte("icc"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		GSPath: fakeGS(t), ICCPath: icc, PDFX: true,
		Timeout: 30 * time.Second, Enabled: true,
	}
	out, err := Convert(context.Background(), cfg, []byte("%PDF-fake"))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if !strings.Contains(string(out), "%GS") {
		t.Fatalf("post-processed bytes wrong: %q", out)
	}
}
