package recovery

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	txstate "github.com/AidarKhusainov/tunwarden/internal/state"
)

func TestDaemonCleanupExecutorSkipsRuntimeRoot(t *testing.T) {
	runtimeDir := t.TempDir()
	marker := filepath.Join(runtimeDir, "marker")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	result := (DaemonCleanupExecutor{RuntimeDir: runtimeDir}).Cleanup(context.Background(), Candidate{
		Kind:        "runtime-directory",
		Description: "runtime directory",
		Target:      runtimeDir,
	})

	if result.Status != "skipped" {
		t.Fatalf("expected runtime root cleanup to be skipped, got %#v", result)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("runtime root child must remain after skipped cleanup: %v", err)
	}
}

func TestDaemonCleanupExecutorSkipsAmbiguousChildProcessMetadata(t *testing.T) {
	runtimeDir := t.TempDir()
	store := txstate.TransactionStore{RuntimeDir: runtimeDir}
	tx := txstate.NewTransaction("tx-child-process", "profile-1", "tun", time.Now().UTC())
	tx.State = txstate.TransactionApplying
	tx.Rollback = txstate.RollbackMetadata{
		ChildProcesses: []txstate.ChildProcessRollback{{
			Label: "xray",
			PID:   424242,
			Owner: txstate.TransactionOwner,
		}},
	}
	path, err := store.Save(tx)
	if err != nil {
		t.Fatalf("save transaction: %v", err)
	}

	result := (DaemonCleanupExecutor{RuntimeDir: runtimeDir}).Cleanup(context.Background(), Candidate{
		Kind:        "transaction-state",
		Description: "transaction rollback state",
		Target:      path,
		Transaction: &TransactionCandidate{
			ID:              tx.ID,
			State:           string(tx.State),
			Status:          "pending apply",
			RequiresCleanup: true,
			Path:            path,
		},
	})

	if result.Status != "skipped" {
		t.Fatalf("expected child process rollback to be skipped, got %#v", result)
	}
	if !strings.Contains(result.Message, "process identity cannot be verified") {
		t.Fatalf("expected explicit child process reason, got %q", result.Message)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("transaction file must remain when cleanup is skipped: %v", err)
	}
}
