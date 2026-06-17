package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/AidarKhusainov/tunwarden/internal/api"
	"github.com/AidarKhusainov/tunwarden/internal/profile"
)

func TestRunCLIConnectRejectsUnsupportedProfileBeforeDaemon(t *testing.T) {
	storePath := t.TempDir() + "/profiles.json"
	p := testConnectProfile()
	p.ID = "amneziawg-profile"
	p.Engine = profile.EngineAmneziaWG
	store, err := profile.NewStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Add(p); err != nil {
		t.Fatal(err)
	}

	calledDaemon := false
	var out bytes.Buffer
	err = runWithOptions(context.Background(), []string{"connect", "--mode", "proxy-only", p.ID}, &out, options{
		profileStorePath: storePath,
		connect: func(context.Context, profile.Profile, string) (api.LifecycleResponse, error) {
			calledDaemon = true
			return api.LifecycleResponse{}, nil
		},
	})
	if err == nil {
		t.Fatal("expected unsupported profile to fail")
	}
	if calledDaemon {
		t.Fatal("unsupported profile was sent to the daemon")
	}
	if !strings.Contains(err.Error(), "proxy-only connect requires engine") {
		t.Fatalf("unexpected error: %v", err)
	}
}
