package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCLIImportFileURLXrayJSONSubscription(t *testing.T) {
	dir := t.TempDir()
	profileStorePath := filepath.Join(dir, "profiles.json")
	fixturePath := filepath.Join(dir, "remote-xray.json")
	userID := uuidForTest(21)
	writeXrayJSONSubscriptionFixture(t, fixturePath, userID, "file-json.example", "file-json", "tcp", "tls")
	opts := options{profileStorePath: profileStorePath}

	var out bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"import", localFileURL(fixturePath)}, &out, opts); err != nil {
		t.Fatalf("Xray JSON subscription import failed: %v", err)
	}
	for _, want := range []string{"Subscription imported:", "Format: xray-json", "Imported: 1", "Unsupported: 0", "Warnings: 0"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("expected import output to contain %q, got %q", want, out.String())
		}
	}

	var profiles bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"profile", "list"}, &profiles, opts); err != nil {
		t.Fatalf("profile list failed: %v", err)
	}
	if got := profiles.String(); !strings.Contains(got, "file-json.example") || strings.Contains(got, userID) {
		t.Fatalf("unexpected profile list output: %q", got)
	}
}

func TestRunCLISubscriptionUpdateFileURLXrayJSONPreservesLastKnownGood(t *testing.T) {
	dir := t.TempDir()
	profileStorePath := filepath.Join(dir, "profiles.json")
	fixturePath := filepath.Join(dir, "remote-xray.json")
	writeXrayJSONSubscriptionFixture(t, fixturePath, uuidForTest(22), "stable-json.example", "stable-json", "tcp", "tls")
	opts := options{profileStorePath: profileStorePath}

	if err := runWithOptions(context.Background(), []string{"subscription", "add", "--name", "remote-json", "--url", localFileURL(fixturePath)}, &bytes.Buffer{}, opts); err != nil {
		t.Fatalf("subscription add failed: %v", err)
	}
	var updateOut bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"subscription", "update", "remote-json"}, &updateOut, opts); err != nil {
		t.Fatalf("subscription update failed: %v", err)
	}
	if !strings.Contains(updateOut.String(), "Format: xray-json") {
		t.Fatalf("expected xray-json update output, got %q", updateOut.String())
	}

	if err := os.WriteFile(fixturePath, []byte(" {not-json"), 0o600); err != nil {
		t.Fatalf("write malformed fixture: %v", err)
	}
	if err := runWithOptions(context.Background(), []string{"subscription", "update", "remote-json"}, &bytes.Buffer{}, opts); err == nil {
		t.Fatal("expected malformed JSON update to fail")
	}

	var profiles bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"profile", "list"}, &profiles, opts); err != nil {
		t.Fatalf("profile list failed: %v", err)
	}
	if got := profiles.String(); !strings.Contains(got, "stable-json.example") || strings.Contains(got, "not-json") {
		t.Fatalf("last-known-good profile state was not preserved: %q", got)
	}
}

func writeXrayJSONSubscriptionFixture(t *testing.T, path, userID, host, tag, network, security string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(cliXrayJSONSubscription(userID, host, tag, network, security)), 0o600); err != nil {
		t.Fatalf("write Xray JSON subscription fixture: %v", err)
	}
}

func cliXrayJSONSubscription(userID, host, tag, network, security string) string {
	return fmt.Sprintf(`{
  "outbounds": [
    {
      "protocol": "vless",
      "tag": %q,
      "settings": {
        "vnext": [
          {
            "address": %q,
            "port": 443,
            "users": [
              {
                "id": %q,
                "encryption": "none"
              }
            ]
          }
        ]
      },
      "streamSettings": {
        "network": %q,
        "security": %q,
        "tlsSettings": {
          "serverName": %q
        }
      }
    }
  ]
}`, tag, host, userID, network, security, host)
}
