package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunImportHTTPSubscriptionFailureDoesNotCreateProfiles(t *testing.T) {
	dir := t.TempDir()
	opts := options{
		profileStorePath:      filepath.Join(dir, "profiles.json"),
		subscriptionStorePath: filepath.Join(dir, "subscriptions.json"),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "subscription unavailable", http.StatusInternalServerError)
	}))
	defer server.Close()

	err := runWithOptions(context.Background(), []string{"import", server.URL + "/sub"}, &bytes.Buffer{}, opts)
	if err == nil {
		t.Fatal("expected import to fail")
	}
	if got := ExitCode(err); got != 1 {
		t.Fatalf("expected runtime exit code 1, got %d", got)
	}
	if !strings.Contains(err.Error(), "unexpected HTTP status 500") {
		t.Fatalf("unexpected import error: %v", err)
	}

	var listOut bytes.Buffer
	if listErr := runWithOptions(context.Background(), []string{"profile", "list"}, &listOut, opts); listErr != nil {
		t.Fatalf("profile list failed: %v", listErr)
	}
	if strings.Contains(listOut.String(), "example.com") || strings.Contains(listOut.String(), "imported") {
		t.Fatalf("failed HTTP import created profile state: %q", listOut.String())
	}
}
