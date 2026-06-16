package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCLIImportHTTPXrayJSONSubscription(t *testing.T) {
	userID := uuidForTest(12)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(remoteCLIXrayJSON(userID, "http-json.example", "http-json", "tcp", "tls")))
	}))
	defer server.Close()

	opts := options{profileStorePath: filepath.Join(t.TempDir(), "profiles.json")}

	var out bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"import", server.URL + "/sub"}, &out, opts); err != nil {
		t.Fatalf("HTTP Xray JSON subscription import failed: %v", err)
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
	if got := profiles.String(); !strings.Contains(got, "http-json.example") {
		t.Fatalf("profile list missing imported profile: %q", got)
	}
}

func TestRunCLISubscriptionUpdateHTTPXrayJSONPreservesLastKnownGoodOnMalformedJSON(t *testing.T) {
	userID := uuidForTest(13)
	response := remoteCLIXrayJSON(userID, "stable-json.example", "stable-json", "tcp", "tls")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	opts := options{profileStorePath: filepath.Join(t.TempDir(), "profiles.json")}
	if err := runWithOptions(context.Background(), []string{"subscription", "add", "--name", "remote-json", "--url", server.URL + "/sub"}, &bytes.Buffer{}, opts); err != nil {
		t.Fatalf("subscription add failed: %v", err)
	}
	var firstUpdate bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"subscription", "update", "remote-json"}, &firstUpdate, opts); err != nil {
		t.Fatalf("subscription update failed: %v", err)
	}
	if !strings.Contains(firstUpdate.String(), "Format: xray-json") {
		t.Fatalf("expected xray-json update output, got %q", firstUpdate.String())
	}

	response = " {definitely-not-json"
	err := runWithOptions(context.Background(), []string{"subscription", "update", "remote-json"}, &bytes.Buffer{}, opts)
	if err == nil {
		t.Fatal("expected malformed JSON update to fail")
	}
	if !strings.Contains(err.Error(), "Xray JSON") || strings.Contains(err.Error(), "Base64") {
		t.Fatalf("expected JSON parse error without Base64 fallback, got %v", err)
	}

	var profiles bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"profile", "list"}, &profiles, opts); err != nil {
		t.Fatalf("profile list failed: %v", err)
	}
	if got := profiles.String(); !strings.Contains(got, "stable-json.example") || strings.Contains(got, "definitely-not-json") {
		t.Fatalf("failed update did not preserve last-known-good profiles: %q", got)
	}
}

func remoteCLIXrayJSON(userID, host, tag, network, security string) string {
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
