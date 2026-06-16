package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCLIImportHTTPBase64SubscriptionStillWorks(t *testing.T) {
	secretToken := "provider-token-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("token"); got != secretToken {
			t.Fatalf("expected token query parameter, got %q", got)
		}
		_, _ = w.Write([]byte(remoteBase64Subscription([]string{
			shareLink(11, "http-base64.example", "443", "?type=tcp&security=tls", "http-base64"),
			unsupportedLink("hy", "steria"),
		})))
	}))
	defer server.Close()

	sourceURL := server.URL + "/sub?token=" + secretToken
	opts := options{profileStorePath: filepath.Join(t.TempDir(), "profiles.json")}

	var out bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"import", sourceURL}, &out, opts); err != nil {
		t.Fatalf("HTTP Base64 subscription import failed: %v", err)
	}
	for _, want := range []string{"Subscription imported:", "Format: base64", "Imported: 1", "Unsupported: 1"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("expected import output to contain %q, got %q", want, out.String())
		}
	}
	for _, leaked := range []string{sourceURL, secretToken, uuidForTest(11)} {
		if strings.Contains(out.String(), leaked) {
			t.Fatalf("HTTP Base64 import leaked %q in output %q", leaked, out.String())
		}
	}
}

func TestRunCLIImportHTTPXrayJSONSubscription(t *testing.T) {
	secretToken := "provider-token-secret"
	userID := uuidForTest(12)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("token"); got != secretToken {
			t.Fatalf("expected token query parameter, got %q", got)
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(remoteCLIXrayJSON(userID, "http-json.example", "http-json", "tcp", "tls")))
	}))
	defer server.Close()

	sourceURL := server.URL + "/sub?token=" + secretToken
	opts := options{profileStorePath: filepath.Join(t.TempDir(), "profiles.json")}

	var out bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"import", sourceURL}, &out, opts); err != nil {
		t.Fatalf("HTTP Xray JSON subscription import failed: %v", err)
	}
	for _, want := range []string{"Subscription imported:", "Format: xray-json", "Imported: 1", "Unsupported: 0", "Warnings: 0"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("expected import output to contain %q, got %q", want, out.String())
		}
	}
	for _, leaked := range []string{sourceURL, secretToken, userID} {
		if strings.Contains(out.String(), leaked) {
			t.Fatalf("HTTP Xray JSON import leaked %q in output %q", leaked, out.String())
		}
	}

	var subscriptions bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"subscription", "list"}, &subscriptions, opts); err != nil {
		t.Fatalf("subscription list failed: %v", err)
	}
	if got := subscriptions.String(); !strings.Contains(got, "imported-subscription-") || strings.Contains(got, secretToken) {
		t.Fatalf("subscription list did not show redacted subscription metadata: %q", got)
	}

	var profiles bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"profile", "list"}, &profiles, opts); err != nil {
		t.Fatalf("profile list failed: %v", err)
	}
	if got := profiles.String(); !strings.Contains(got, "http-json.example") || strings.Contains(got, userID) {
		t.Fatalf("profile list missing imported profile or leaked identity: %q", got)
	}
}

func TestRunCLISubscriptionUpdateHTTPXrayJSONPreservesLastKnownGoodOnMalformedJSON(t *testing.T) {
	secretToken := "provider-token-secret"
	userID := uuidForTest(13)
	response := remoteCLIXrayJSON(userID, "stable-json.example", "stable-json", "tcp", "tls")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("token"); got != secretToken {
			t.Fatalf("expected token query parameter, got %q", got)
		}
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	sourceURL := server.URL + "/sub?token=" + secretToken
	opts := options{profileStorePath: filepath.Join(t.TempDir(), "profiles.json")}
	if err := runWithOptions(context.Background(), []string{"subscription", "add", "--name", "remote-json", "--url", sourceURL}, &bytes.Buffer{}, opts); err != nil {
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
	for _, leaked := range []string{sourceURL, secretToken, userID} {
		if strings.Contains(err.Error(), leaked) {
			t.Fatalf("malformed update error leaked %q in %q", leaked, err.Error())
		}
	}

	var profiles bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"profile", "list"}, &profiles, opts); err != nil {
		t.Fatalf("profile list failed: %v", err)
	}
	if got := profiles.String(); !strings.Contains(got, "stable-json.example") || strings.Contains(got, "definitely-not-json") {
		t.Fatalf("failed update did not preserve last-known-good profiles: %q", got)
	}
}

func TestRunCLIImportHTTPPlaceholderXrayJSONFailsWithoutPersisting(t *testing.T) {
	secretToken := "provider-token-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(remoteCLIXrayJSON("not-a-real-client", "placeholder-json.example", "App not supported", "tcp", "tls")))
	}))
	defer server.Close()

	sourceURL := server.URL + "/sub?token=" + secretToken
	opts := options{profileStorePath: filepath.Join(t.TempDir(), "profiles.json")}
	err := runWithOptions(context.Background(), []string{"import", sourceURL}, &bytes.Buffer{}, opts)
	if err == nil {
		t.Fatal("expected placeholder JSON import to fail")
	}
	for _, want := range []string{"Xray JSON", "no supported", "user id must be a UUID"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected error containing %q, got %v", want, err)
		}
	}
	for _, leaked := range []string{sourceURL, secretToken, "not-a-real-client"} {
		if strings.Contains(err.Error(), leaked) {
			t.Fatalf("placeholder import error leaked %q in %q", leaked, err.Error())
		}
	}

	var profiles bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"profile", "list"}, &profiles, opts); err != nil {
		t.Fatalf("profile list failed: %v", err)
	}
	if strings.Contains(profiles.String(), "placeholder-json.example") {
		t.Fatalf("failed placeholder import persisted profile: %q", profiles.String())
	}

	var subscriptions bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"subscription", "list"}, &subscriptions, opts); err != nil {
		t.Fatalf("subscription list failed: %v", err)
	}
	if strings.Contains(subscriptions.String(), "imported-subscription-") {
		t.Fatalf("failed placeholder import persisted subscription: %q", subscriptions.String())
	}
}

func remoteBase64Subscription(entries []string) string {
	return base64.StdEncoding.EncodeToString([]byte(strings.Join(entries, "\n")))
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
