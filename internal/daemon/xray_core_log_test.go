package daemon

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestCoreLogWriterFlushesFinalLineWithoutTrailingNewline(t *testing.T) {
	var out bytes.Buffer
	originalOutput := log.Writer()
	originalFlags := log.Flags()
	log.SetOutput(&out)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(originalOutput)
		log.SetFlags(originalFlags)
	}()

	writer := newCoreLogWriter("test-profile", "stderr")
	writer.setPID(42)
	if _, err := writer.Write([]byte("final crash line without newline")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	writer.Flush()

	got := out.String()
	for _, text := range []string{"tunwardend: core xray stderr", "pid=42", "profile=test-profile", "final crash line without newline"} {
		if !strings.Contains(got, text) {
			t.Fatalf("expected core log output to contain %q, got %q", text, got)
		}
	}
}

func TestCoreLogWriterSplitsCompleteLinesAndFlushesTail(t *testing.T) {
	var out bytes.Buffer
	originalOutput := log.Writer()
	originalFlags := log.Flags()
	log.SetOutput(&out)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(originalOutput)
		log.SetFlags(originalFlags)
	}()

	writer := newCoreLogWriter("test-profile", "stdout")
	writer.setPID(43)
	if _, err := writer.Write([]byte("first line\nsecond line without newline")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	writer.Flush()

	got := out.String()
	for _, text := range []string{"first line", "second line without newline"} {
		if !strings.Contains(got, text) {
			t.Fatalf("expected core log output to contain %q, got %q", text, got)
		}
	}
	if count := strings.Count(got, "tunwardend: core xray stdout"); count != 2 {
		t.Fatalf("expected two core stdout log lines, got %d: %q", count, got)
	}
}
