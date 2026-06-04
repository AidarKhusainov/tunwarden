package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/AidarKhusainov/tunwarden/internal/doctor"
)

func TestRunCLIDoctorCoreRendersXrayDiagnostics(t *testing.T) {
	var out bytes.Buffer
	var gotPath string

	err := runWithOptions(context.Background(), []string{"doctor", "--core", "--xray", "/usr/local/bin/xray"}, &out, options{
		coreDoctor: func(_ context.Context, xrayPath string) doctor.Report {
			gotPath = xrayPath
			return cleanCoreDoctorReport(xrayPath)
		},
	})
	if err != nil {
		t.Fatalf("doctor --core failed: %v", err)
	}
	if gotPath != "/usr/local/bin/xray" {
		t.Fatalf("expected xray path to be passed through, got %q", gotPath)
	}
	got := out.String()
	for _, text := range []string{"TunWarden core diagnostics", "[OK] xray: /usr/local/bin/xray is executable", "[OK] xray-version: Xray 25.6.1", "[WARN] config-test: not checked"} {
		if !strings.Contains(got, text) {
			t.Fatalf("expected output to contain %q, got %q", text, got)
		}
	}
}

func TestRunCLIDoctorCoreSupportsInlineXrayPathAndJSON(t *testing.T) {
	var out bytes.Buffer

	err := runWithOptions(context.Background(), []string{"doctor", "--core", "--xray=/usr/local/bin/xray", "--json"}, &out, options{
		coreDoctor: func(_ context.Context, xrayPath string) doctor.Report {
			return cleanCoreDoctorReport(xrayPath)
		},
	})
	if err != nil {
		t.Fatalf("doctor --core --json failed: %v", err)
	}

	got := out.String()
	for _, text := range []string{"\"schema_version\": \"v1\"", "\"status\": \"warn\"", "\"source\": \"local core\"", "\"name\": \"xray-version\"", "\"not checked\""} {
		if !strings.Contains(got, text) {
			t.Fatalf("expected JSON output to contain %q, got %q", text, got)
		}
	}
}

func TestRunCLIDoctorCoreReturnsDiagnosticExitCodeForFailures(t *testing.T) {
	var out bytes.Buffer

	err := runWithOptions(context.Background(), []string{"doctor", "--core", "--xray", "/missing/xray"}, &out, options{
		coreDoctor: func(_ context.Context, xrayPath string) doctor.Report {
			return doctor.Report{Source: doctor.SourceLocalCore, Checks: []doctor.Check{{Name: "xray", Severity: doctor.SeverityFail, Message: xrayPath + " does not exist; install Xray or pass the correct --xray path"}}}
		},
	})
	if err == nil {
		t.Fatal("expected doctor --core to fail when xray check fails")
	}
	if got := ExitCode(err); got != 3 {
		t.Fatalf("expected doctor --core diagnostic exit code 3, got %d", got)
	}
	if got := out.String(); !strings.Contains(got, "[FAIL] xray") || !strings.Contains(got, "does not exist") {
		t.Fatalf("expected failing xray diagnostic in output, got %q", got)
	}
}

func TestRunCLIDoctorCoreRejectsMissingXrayValue(t *testing.T) {
	var out bytes.Buffer
	err := run(context.Background(), []string{"doctor", "--core", "--xray"}, &out)
	if err == nil {
		t.Fatal("expected missing --xray value to fail")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("expected exit code 2, got %d", got)
	}
	if !strings.Contains(err.Error(), "doctor --core --xray requires a value") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func cleanCoreDoctorReport(xrayPath string) doctor.Report {
	return doctor.Report{Source: doctor.SourceLocalCore, Checks: []doctor.Check{
		{Name: "xray", Severity: doctor.SeverityOK, Message: xrayPath + " is executable"},
		{Name: "xray-version", Severity: doctor.SeverityOK, Message: "Xray 25.6.1 (Xray, Penetrates Everything.)"},
		{Name: "config-test", Severity: doctor.SeverityWarning, Message: "not checked"},
	}}
}
