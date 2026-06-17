package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AidarKhusainov/tunwarden/internal/profile"
)

func TestRunCLIProfileValidateJSONRedactsRenderableProfile(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "profiles.json")
	store, err := profile.NewStore(storePath)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	p := renderableVLESSProfile()
	if err := store.Add(p); err != nil {
		t.Fatalf("add profile: %v", err)
	}

	var out bytes.Buffer
	err = runWithOptions(context.Background(), []string{"profile", "validate", p.ID, "--mode", "proxy-only", "--json"}, &out, options{profileStorePath: storePath})
	if err != nil {
		t.Fatalf("profile validate --json failed: %v", err)
	}

	got := out.String()
	for _, want := range []string{`"schema_version": "v1"`, `"status": "ok"`, `"valid": true`, `"mode": "proxy-only"`, `"backend": "xray"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected JSON output to contain %q, got %q", want, got)
		}
	}
	if strings.Contains(got, p.UserIdentity) {
		t.Fatalf("expected user identity to be redacted, got %q", got)
	}
}

func TestRunCLIProfileValidateJSONReturnsDiagnosticExitCodeForUnsupportedBackendProfile(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "profiles.json")
	store, err := profile.NewStore(storePath)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	p := renderableVLESSProfile()
	p.ID = "vmess-profile"
	p.Name = "vmess profile"
	p.Protocol = "vmess"
	if err := store.Add(p); err != nil {
		t.Fatalf("add profile: %v", err)
	}

	var out bytes.Buffer
	err = runWithOptions(context.Background(), []string{"profile", "validate", p.ID, "--mode", "tun", "--json"}, &out, options{profileStorePath: storePath})
	if err == nil {
		t.Fatal("expected profile validate to return diagnostic exit code")
	}
	if got := ExitCode(err); got != 3 {
		t.Fatalf("expected diagnostic exit code 3, got %d", got)
	}

	got := out.String()
	for _, want := range []string{`"status": "fail"`, `"valid": false`, `"mode": "tun"`, "VLESS profiles only"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected JSON output to contain %q, got %q", want, got)
		}
	}
	if strings.Contains(got, p.UserIdentity) {
		t.Fatalf("expected user identity to be redacted, got %q", got)
	}
}

func TestRunCLIProfileValidateRejectsUnsupportedMode(t *testing.T) {
	var out bytes.Buffer
	err := runWithOptions(context.Background(), []string{"profile", "validate", "demo", "--mode", "bad"}, &out, options{profileStorePath: filepath.Join(t.TempDir(), "profiles.json")})
	assertUsageError(t, err, out.String(), "unsupported profile validate mode")
}

func renderableVLESSProfile() profile.Profile {
	return profile.Profile{
		ID:           "demo-vless",
		Name:         "demo vless",
		Source:       profile.SourceImportedURI,
		Engine:       profile.EngineXray,
		Server:       "example.com",
		Port:         443,
		Protocol:     "vless",
		UserIdentity: "00000000-0000-0000-0000-000000000000",
		Transport:    "tcp",
		Security:     "none",
		Encryption:   "none",
	}
}
