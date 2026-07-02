package check

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSelectTargetsDefaultsToConservativeCatalog(t *testing.T) {
	targets, err := SelectTargets(nil)
	if err != nil {
		t.Fatalf("SelectTargets failed: %v", err)
	}
	if len(targets) != 1 || targets[0].ID != "cloudflare" {
		t.Fatalf("expected conservative cloudflare default, got %#v", targets)
	}
}

func TestSelectTargetsRejectsUnknownTarget(t *testing.T) {
	_, err := SelectTargets([]string{"telegram", "unknown"})
	if err == nil {
		t.Fatal("expected unknown target to fail")
	}
	if !strings.Contains(err.Error(), "unknown check target") || !strings.Contains(err.Error(), "telegram") {
		t.Fatalf("expected supported target list in error, got %v", err)
	}
}

func TestSelectTargetsDeduplicatesAndSorts(t *testing.T) {
	targets, err := SelectTargets([]string{"github", "telegram", "github"})
	if err != nil {
		t.Fatalf("SelectTargets failed: %v", err)
	}
	if got := []string{targets[0].ID, targets[1].ID}; len(targets) != 2 || got[0] != "github" || got[1] != "telegram" {
		t.Fatalf("expected sorted unique targets, got %#v", targets)
	}
}

func TestCatalogDefinesLowImpactHTTPSProbes(t *testing.T) {
	for _, target := range Catalog() {
		if target.ID == "" || target.DisplayName == "" {
			t.Fatalf("target has missing identity: %#v", target)
		}
		if target.ProbeType != ProbeTypeHTTPS {
			t.Fatalf("target %q uses unsupported test probe type %q", target.ID, target.ProbeType)
		}
		if !strings.HasPrefix(target.URL, "https://") {
			t.Fatalf("target %q must use HTTPS URL, got %q", target.ID, target.URL)
		}
		if target.SuccessCondition == "" || target.PrivacyNote == "" {
			t.Fatalf("target %q must document success and privacy notes", target.ID)
		}
	}
}

func TestProbeHTTPSClassifiesHTTPResults(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantStatus string
	}{
		{name: "success", statusCode: http.StatusNoContent, wantStatus: ResultOK},
		{name: "server error", statusCode: http.StatusServiceUnavailable, wantStatus: ResultFail},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("User-Agent") != "podlaz-check/0" {
					t.Fatalf("unexpected User-Agent %q", r.Header.Get("User-Agent"))
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			result := probeHTTPS(context.Background(), server.Client(), Target{
				ID:               "test",
				DisplayName:      "Test",
				ProbeType:        ProbeTypeHTTPS,
				URL:              server.URL,
				SuccessCondition: "HTTP status below 500",
			}, DefaultTimeout)
			if result.Status != tt.wantStatus {
				t.Fatalf("expected %s, got %#v", tt.wantStatus, result)
			}
		})
	}
}
