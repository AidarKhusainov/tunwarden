package logs

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunJournalctlFollowStopsWhenContextExpires(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake journalctl script uses POSIX shell")
	}

	dir := t.TempDir()
	argsFile := filepath.Join(dir, "journalctl.args")
	journalctl := filepath.Join(dir, "journalctl")
	script := "#!/usr/bin/env bash\n" +
		"printf '%s\\n' \"$@\" >\"${JOURNALCTL_ARGS_FILE}\"\n" +
		"printf 'podlazd.service: follow line\\n'\n" +
		"exec sleep 30\n"
	if err := os.WriteFile(journalctl, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake journalctl: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("JOURNALCTL_ARGS_FILE", argsFile)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	started := time.Now()
	var out bytes.Buffer
	err := RunJournalctl(ctx, &out, Options{Follow: true})
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("follow command was not bounded by context timeout; elapsed=%s err=%v", elapsed, err)
	}
	if !strings.Contains(out.String(), "follow line") {
		t.Fatalf("expected fake journalctl output to be rendered before cancellation, got %q", out.String())
	}
	args, readErr := os.ReadFile(argsFile)
	if readErr != nil {
		t.Fatalf("read fake journalctl args: %v", readErr)
	}
	if !strings.Contains(string(args), "--follow") {
		t.Fatalf("expected journalctl args to include --follow, got %q", string(args))
	}
}
