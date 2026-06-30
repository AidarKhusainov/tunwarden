package storejson

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type testStoreFile struct {
	SchemaVersion string   `json:"schema_version"`
	Items         []string `json:"items"`
}

func TestWriteFileWritesIndentedJSONWithRestrictivePermissionsAndParentSync(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "state", "store.json")

	syncCalled := false
	syncParentDir := func(gotDir string) error {
		syncCalled = true
		if gotDir != filepath.Dir(path) {
			t.Fatalf("sync dir = %q, want %q", gotDir, filepath.Dir(path))
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("store was not renamed before parent directory sync: %v", err)
		}
		return nil
	}

	if err := WriteFile(path, testStoreFile{SchemaVersion: "v1", Items: []string{"one"}}, Options{TempPattern: ".store-*.tmp", SyncParentDir: syncParentDir}); err != nil {
		t.Fatalf("write store: %v", err)
	}
	if !syncCalled {
		t.Fatal("expected parent directory sync")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	want := "{\n  \"schema_version\": \"v1\",\n  \"items\": [\n    \"one\"\n  ]\n}\n"
	if string(data) != want {
		t.Fatalf("unexpected JSON encoding:\n%s", data)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat store: %v", err)
	}
	if got := info.Mode().Perm(); got != DefaultFileMode {
		t.Fatalf("expected store mode %o, got %o", DefaultFileMode, got)
	}

	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat store directory: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != DefaultDirectoryMode {
		t.Fatalf("expected store directory mode %o, got %o", DefaultDirectoryMode, got)
	}
}

func TestWriteFileKeepsRenamedStoreWhenParentSyncFailsAndCleansTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")
	injectedErr := errors.New("forced sync failure")

	err := WriteFile(path, testStoreFile{SchemaVersion: "v1", Items: []string{"one"}}, Options{
		TempPattern: ".store-*.tmp",
		SyncParentDir: func(string) error {
			return injectedErr
		},
	})
	if err == nil {
		t.Fatal("expected parent directory sync failure")
	}
	if !errors.Is(err, injectedErr) {
		t.Fatalf("expected injected error, got %v", err)
	}
	var writeErr *WriteError
	if !errors.As(err, &writeErr) {
		t.Fatalf("expected WriteError, got %T", err)
	}
	if writeErr.Operation != OperationSyncParentDirectory {
		t.Fatalf("expected sync parent operation, got %q", writeErr.Operation)
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("expected renamed store to remain readable: %v", readErr)
	}
	var decoded testStoreFile
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("expected renamed store to contain valid JSON: %v", err)
	}
	if decoded.SchemaVersion != "v1" || len(decoded.Items) != 1 || decoded.Items[0] != "one" {
		t.Fatalf("unexpected decoded store: %#v", decoded)
	}

	leftovers, err := filepath.Glob(filepath.Join(dir, ".store-*.tmp"))
	if err != nil {
		t.Fatalf("glob temporary stores: %v", err)
	}
	if len(leftovers) != 0 {
		t.Fatalf("expected temporary store cleanup, found %v", leftovers)
	}
}

func TestWriteFileReportsJSONEncodingFailureBeforeCreatingStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")

	err := WriteFile(path, map[string]any{"bad": make(chan struct{})}, Options{})
	if err == nil {
		t.Fatal("expected JSON encoding failure")
	}
	var writeErr *WriteError
	if !errors.As(err, &writeErr) {
		t.Fatalf("expected WriteError, got %T", err)
	}
	if writeErr.Operation != OperationEncodeJSON {
		t.Fatalf("expected encode operation, got %q", writeErr.Operation)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected no store after encode failure, stat err = %v", statErr)
	}
}
