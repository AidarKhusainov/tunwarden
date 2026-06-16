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
	body := base64.StdEncoding.EncodeToString([]byte(shareLink(20, "http-base64.example", "443", "?type=tcp&security=tls&encryption=none", "http-base64")))
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
	if got := out.String(); !strings.Contains(got, "Format: base64") || strings.Contains(got, secretToken) || strings.Contains(got, uuidForTest(20)) {
		t.Fatalf("unexpected HTTP Base64 import output: %q", got)
	}

	assertSubscriptionJSONContainsFormat(t, opts, []string{"subscription", "list", "--json"}, "base64", secretToken)
}

func TestRunCLIImportHTTPXrayJSONSubscriptionPersistsFormat(t *testing.T) {
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
	if got := out.String(); !strings.Contains(got, "Format: xray-json") || strings.Contains(got, secretToken) || strings.Contains(got, userID) {
		t.Fatalf("unexpected HTTP Xray JSON import output: %q", got)
	}

	assertSubscriptionJSONContainsFormat(t, opts, []string{"subscription", "list", "--json"}, "xray-json", secretToken)

	var profiles bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"profile", "list"}, &profiles, opts); err != nil {
		t.Fatalf("profile list failed: %v", err)
	}
	if got := profiles.String(); !strings.Contains(got, "http-json.example") || strings.Contains(got, userID) {
		t.Fatalf("unexpected profile list output: %q", got)
	}
}

func TestRunCLISubscriptionUpdateHTTPXrayJSONPreservesLastKnownGood(t *testing.T) {
	body := cliXrayJSONSubscription(uuidForTest(22), "stable-json.example", "stable-json", "tcp", "tls")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	opts := options{profileStorePath: filepath.Join(t.TempDir(), "profiles.json")}
	if err := runWithOptions(context.Background(), []string{"subscription", "add", "--name", "remote-json", "--url", server.URL + "/sub"}, &bytes.Buffer{}, opts); err != nil {
		t.Fatalf("subscription add failed: %v", err)
	}
	var updateOut bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"subscription", "update", "remote-json"}, &updateOut, opts); err != nil {
		t.Fatalf("subscription update failed: %v", err)
	}
	if !strings.Contains(updateOut.String(), "Format: xray-json") {
		t.Fatalf("expected xray-json update output, got %q", updateOut.String())
	}
	assertSubscriptionJSONContainsFormat(t, opts, []string{"subscription", "show", "remote-json", "--json"}, "xray-json", "")

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
	if !strings.Contains(out.String(), "Format: xray-json") {
		t.Fatalf("expected xray-json import output, got %q", out.String())
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

func assertSubscriptionJSONContainsFormat(t *testing.T, opts options, args []string, format string, notContains string) {
	t.Helper()
	var out bytes.Buffer
	if err := runWithOptions(context.Background(), args, &out, opts); err != nil {
		t.Fatalf("%s failed: %v", strings.Join(args, " "), err)
	}
	got := out.String()
	if !strings.Contains(got, `"format": "`+format+`"`) || !strings.Contains(got, `"url": "REDACTED"`) {
		t.Fatalf("expected redacted persisted format %q in output: %q", format, got)
	}
	if notContains != "" && strings.Contains(got, notContains) {
		t.Fatalf("subscription json leaked %q in %q", notContains, got)
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
