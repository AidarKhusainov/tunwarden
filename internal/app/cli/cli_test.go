package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/AidarKhusainov/tunwarden/internal/doctor"
)

func TestRunCLIVersion(t *testing.T) {
	var out bytes.Buffer

	err := run(context.Background(), []string{"version"}, &out)
	if err != nil {
		t.Fatalf("version failed: %v", err)
	}

	if got := out.String(); !strings.Contains(got, "tunwarden") {
		t.Fatalf("expected version output to contain binary name, got %q", got)
	}
}

func TestRunCLIUnknownCommand(t *testing.T) {
	var out bytes.Buffer

	err := run(context.Background(), []string{"unknown"}, &out)
	if err == nil {
		t.Fatal("expected unknown command to fail")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("expected unknown command exit code 2, got %d", got)
	}
}

func TestRunCLIDoctorReturnsDiagnosticExitCodeForFailures(t *testing.T) {
	var out bytes.Buffer

	err := runWithOptions(context.Background(), []string{"doctor"}, &out, options{
		doctor: func(context.Context) doctor.Report {
			return doctor.Report{Checks: []doctor.Check{{
				Name:     "default-route",
				Severity: doctor.SeverityFail,
				Message:  "ip route show default failed: test failure",
			}}}
		},
	})

	if err == nil {
		t.Fatal("expected doctor to fail when report has failing checks")
	}
	if got := ExitCode(err); got != 3 {
		t.Fatalf("expected doctor diagnostic exit code 3, got %d", got)
	}
	if got := out.String(); !strings.Contains(got, "[FAIL] default-route") {
		t.Fatalf("expected failing diagnostic in output, got %q", got)
	}
}
