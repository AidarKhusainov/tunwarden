package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCLIProfileAddListAndShow(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "profiles.json")
	opts := options{profileStorePath: storePath}

	var addOut bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"profile", "add", "--name", "test", "--server", "example.com", "--port", "443", "--protocol", "vless"}, &addOut, opts); err != nil {
		t.Fatalf("profile add failed: %v", err)
	}
	if addOut.String() != "Profile added: test\n" {
		t.Fatalf("unexpected add output: %q", addOut.String())
	}

	var listOut bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"profile", "list"}, &listOut, opts); err != nil {
		t.Fatalf("profile list failed: %v", err)
	}
	if got := listOut.String(); !strings.Contains(got, "test") || !strings.Contains(got, "vless") || !strings.Contains(got, "example.com") {
		t.Fatalf("unexpected list output: %q", got)
	}

	var showOut bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"profile", "show", "test"}, &showOut, opts); err != nil {
		t.Fatalf("profile show failed: %v", err)
	}
	if got := showOut.String(); !strings.Contains(got, "ID: test") || !strings.Contains(got, "Source: manual") || !strings.Contains(got, "Port: 443") {
		t.Fatalf("unexpected show output: %q", got)
	}
}
