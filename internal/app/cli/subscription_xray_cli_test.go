package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCLIImportHTTPBase64SubscriptionPersistsFormat(t *testing.T) {
	body := base64.StdEncoding.EncodeToString([]byte(strings.Join([]string{
		shareLink(20, "http-base64.example", "443", "?type=tcp&security=tls&encryption=none", "http-base64"),
		unsupportedLink("hy", "steria"),
	}, "\n")))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	secretToken := "provider-token-secret"
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
	for _, leaked := range []string{sourceURL, secretToken, uuidForTest(20)} {
		if strings.Contains(out.String(), leaked) {
			t.Fatalf("HTTP Base64 import leaked %q in output %q", leaked, out.String())
		}
	}

	var subscriptions bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"subscription", "list", "--json"}, &subscriptions, opts); err != nil {
		t.Fatalf("subscription list --json failed: %v", err)
	}
	if got := subscriptions.String(); !strings.Contains(got, `"format": "base64"`) || strings.Contains(got, secretToken) {
		t.Fatalf("subscription list json did not persist redacted Base64 format metadata: %q", got)
	}
}

func TestRunCLIImportHTTPXrayJSONObjectSubscriptionPersistsFormat(t *testing.T) {
	userID := uuidForTest(21)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(cliXrayJSONSubscription(userID, "http-json.example", "http-json", "tcp", "tls")))
	}))
	defer server.Close()

	secretToken := "provider-token-secret"
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
	if err := runWithOptions(context.Background(), []string{"subscription", "list", "--json"}, &subscriptions, opts); err != nil {
		t.Fatalf("subscription list --json failed: %v", err)
	}
	if got := subscriptions.String(); !strings.Contains(got, `"format": "xray-json"`) || strings.Contains(got, secretToken) {
		t.Fatalf("subscription list json did not persist redacted Xray JSON format metadata: %q", got)
	}

	var profiles bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"profile", "list"}, &profiles, opts); err != nil {
		t.Fatalf("profile list failed: %v", err)
	}
	if got := profiles.String(); !strings.Contains(got, "http-json.example") || strings.Contains(got, userID) {
		t.Fatalf("unexpected profile list output: %q", got)
	}
}

func TestRunCLISubscriptionUpdateHTTPXrayJSONArrayPersistsFormat(t *testing.T) {
	body := "[" +
		cliXrayJSONSubscription(uuidForTest(22), "array-one.example", "array-one", "tcp", "tls") + "," +
		cliXrayJSONSubscription(uuidForTest(23), "array-two.example", "array-two", "grpc", "reality") +
		"]"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	secretToken := "provider-token-secret"
	sourceURL := server.URL + "/sub?token=" + secretToken
	opts := options{profileStorePath: filepath.Join(t.TempDir(), "profiles.json")}

	if err := runWithOptions(context.Background(), []string{"subscription", "add", "--name", "remote-json", "--url", sourceURL}, &bytes.Buffer{}, opts); err != nil {
		t.Fatalf("subscription add failed: %v", err)
	}
	var updateOut bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"subscription", "update", "remote-json"}, &updateOut, opts); err != nil {
		t.Fatalf("subscription update failed: %v", err)
	}
	for _, want := range []string{"Subscription updated: remote-json", "Format: xray-json", "Imported: 2", "Unsupported: 0"} {
		if !strings.Contains(updateOut.String(), want) {
			t.Fatalf("expected update output to contain %q, got %q", want, updateOut.String())
		}
	}

	var showJSON bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"subscription", "show", "remote-json", "--json"}, &showJSON, opts); err != nil {
		t.Fatalf("subscription show --json failed: %v", err)
	}
	if got := showJSON.String(); !strings.Contains(got, `"format": "xray-json"`) || !strings.Contains(got, `"url": "REDACTED"`) || strings.Contains(got, secretToken) {
		t.Fatalf("subscription show json did not persist redacted Xray JSON format metadata: %q", got)
	}

	var profiles bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"profile", "list"}, &profiles, opts); err != nil {
		t.Fatalf("profile list failed: %v", err)
	}
	for _, want := range []string{"array-one.example", "array-two.example"} {
		if !strings.Contains(profiles.String(), want) {
			t.Fatalf("expected profile list to contain %q, got %q", want, profiles.String())
		}
	}
}

func TestRunCLISubscriptionUpdateHTTPMalformedJSONPreservesLastKnownGood(t *testing.T) {
	body := cliXrayJSONSubscription(uuidForTest(24), "stable-json.example", "stable-json", "tcp", "tls")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	opts := options{profileStorePath: filepath.Join(t.TempDir(), "profiles.json")}
	if err := runWithOptions(context.Background(), []string{"subscription", "add", "--name", "remote-json", "--url", server.URL + "/sub"}, &bytes.Buffer{}, opts); err != nil {
		t.Fatalf("subscription add failed: %v", err)
	}
	if err := runWithOptions(context.Background(), []string{"subscription", "update", "remote-json"}, &bytes.Buffer{}, opts); err != nil {
		t.Fatalf("subscription update failed: %v", err)
	}

	body = " {definitely-not-json"
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
		t.Fatalf("last-known-good profile state was not preserved: %q", got)
	}
}

func TestRunCLIImportHTTPPlaceholderXrayJSONFailsWithoutPersisting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(cliXrayJSONSubscription("not-a-real-client", "placeholder-json.example", "App not supported", "tcp", "tls")))
	}))
	defer server.Close()

	secretToken := "provider-token-secret"
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

func TestRunCLIImportFileURLXrayJSONSubscription(t *testing.T) {
	dir := t.TempDir()
	profileStorePath := filepath.Join(dir, "profiles.json")
	fixturePath := filepath.Join(dir, "remote-xray.json")
	userID := uuidForTest(25)
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
	writeXrayJSONSubscriptionFixture(t, fixturePath, uuidForTest(26), "stable-file-json.example", "stable-file-json", "tcp", "tls")
	opts := options{profileStorePath: profileStorePath}

	if err := runWithOptions(context.Background(), []string{"subscription", "add", "--name", "remote-file-json", "--url", localFileURL(fixturePath)}, &bytes.Buffer{}, opts); err != nil {
		t.Fatalf("subscription add failed: %v", err)
	}
	var updateOut bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"subscription", "update", "remote-file-json"}, &updateOut, opts); err != nil {
		t.Fatalf("subscription update failed: %v", err)
	}
	if !strings.Contains(updateOut.String(), "Format: xray-json") {
		t.Fatalf("expected xray-json update output, got %q", updateOut.String())
	}

	if err := os.WriteFile(fixturePath, []byte(" {not-json"), 0o600); err != nil {
		t.Fatalf("write malformed fixture: %v", err)
	}
	if err := runWithOptions(context.Background(), []string{"subscription", "update", "remote-file-json"}, &bytes.Buffer{}, opts); err == nil {
		t.Fatal("expected malformed JSON update to fail")
	}

	var profiles bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"profile", "list"}, &profiles, opts); err != nil {
		t.Fatalf("profile list failed: %v", err)
	}
	if got := profiles.String(); !strings.Contains(got, "stable-file-json.example") || strings.Contains(got, "not-json") {
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
        },
        "realitySettings": {
          "serverName": %q,
          "publicKey": "public-key",
          "shortId": "abcd"
        },
        "grpcSettings": {
          "serviceName": "svc"
        }
      }
    }
  ]
}`, tag, host, userID, network, security, host, host)
}
