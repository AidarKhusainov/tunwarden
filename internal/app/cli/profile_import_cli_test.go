package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCLIProfileImportVLESSListAndShow(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "profiles.json")
	opts := options{profileStorePath: storePath}
	uri := "vless://00000000-0000-0000-0000-000000000001@example.com:443?type=tcp&security=reality&encryption=none&flow=xtls-rprx-vision&sni=example.com&fp=chrome&pbk=public-key&sid=abcd&spx=%2F#my-vless-profile"

	var importOut bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"profile", "import", uri}, &importOut, opts); err != nil {
		t.Fatalf("profile import failed: %v", err)
	}
	if got := importOut.String(); !strings.Contains(got, "Imported profile: my-vless-profile") || !strings.Contains(got, "Warnings: 1") || !strings.Contains(got, "flow is preserved") {
		t.Fatalf("unexpected import output: %q", got)
	}
	if strings.Contains(importOut.String(), "00000000-0000-0000-0000-000000000001") {
		t.Fatalf("import output leaked VLESS user identity: %q", importOut.String())
	}

	var listOut bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"profile", "list"}, &listOut, opts); err != nil {
		t.Fatalf("profile list failed: %v", err)
	}
	if got := listOut.String(); !strings.Contains(got, "my-vless-profile") || !strings.Contains(got, "vless") || !strings.Contains(got, "example.com") {
		t.Fatalf("unexpected list output: %q", got)
	}

	var showOut bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"profile", "show", "my-vless-profile"}, &showOut, opts); err != nil {
		t.Fatalf("profile show failed: %v", err)
	}
	show := showOut.String()
	for _, want := range []string{"Source: imported_uri", "User identity: 0000…0001", "Security: reality", "Flow: xtls-rprx-vision", "Reality public key: public-key"} {
		if !strings.Contains(show, want) {
			t.Fatalf("expected profile show to contain %q, got %q", want, show)
		}
	}
	if strings.Contains(show, "00000000-0000-0000-0000-000000000001") {
		t.Fatalf("profile show leaked full VLESS user identity: %q", show)
	}
}

func TestRunCLIProfileImportRejectsMalformedVLESS(t *testing.T) {
	err := runWithOptions(context.Background(), []string{"profile", "import", "vless://00000000-0000-0000-0000-000000000001@example.com?type=tcp#missing-port"}, &bytes.Buffer{}, options{profileStorePath: filepath.Join(t.TempDir(), "profiles.json")})
	if err == nil {
		t.Fatal("expected malformed import to fail")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("expected exit code 2, got %d", got)
	}
	if !strings.Contains(err.Error(), "server port is required") {
		t.Fatalf("unexpected import error: %v", err)
	}
}

func TestRunCLIProfileImportJSONIsDeferred(t *testing.T) {
	err := runWithOptions(context.Background(), []string{"profile", "import", "--json", "vless://demo@example.com:443#demo"}, &bytes.Buffer{}, options{profileStorePath: filepath.Join(t.TempDir(), "profiles.json")})
	if err == nil {
		t.Fatal("expected profile import --json to fail")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("expected exit code 2, got %d", got)
	}
	if !strings.Contains(err.Error(), "profile import --json is not implemented") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCLIProfileShowJSONRedactsImportedUserIdentity(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "profiles.json")
	opts := options{profileStorePath: storePath}
	uri := "vless://00000000-0000-0000-0000-000000000001@example.com:443?type=tcp&security=tls&encryption=none#json-redaction"
	if err := runWithOptions(context.Background(), []string{"profile", "import", uri}, &bytes.Buffer{}, opts); err != nil {
		t.Fatalf("profile import failed: %v", err)
	}

	var out bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"profile", "show", "json-redaction", "--json"}, &out, opts); err != nil {
		t.Fatalf("profile show --json failed: %v", err)
	}
	if strings.Contains(out.String(), "00000000-0000-0000-0000-000000000001") {
		t.Fatalf("profile show --json leaked full VLESS user identity: %q", out.String())
	}
	if !strings.Contains(out.String(), "0000…0001") {
		t.Fatalf("profile show --json did not include redacted identity: %q", out.String())
	}

	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode show JSON: %v", err)
	}
	assertCommonJSON(t, got)
}
