package sub

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchSourceSendsTunWardenUserAgent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.UserAgent(); got != subscriptionUserAgent {
			t.Fatalf("expected User-Agent %q, got %q", subscriptionUserAgent, got)
		}
		_, _ = w.Write([]byte("subscription"))
	}))
	defer server.Close()

	data, err := FetchSource(context.Background(), Source{ID: "ua", Name: "ua", URL: server.URL + "/sub", Format: FormatBase64})
	if err != nil {
		t.Fatalf("FetchSource failed: %v", err)
	}
	if string(data) != "subscription" {
		t.Fatalf("unexpected body: %q", data)
	}
}
