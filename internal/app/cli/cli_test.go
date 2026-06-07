package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/AidarKhusainov/tunwarden/internal/doctor"
	"github.com/AidarKhusainov/tunwarden/internal/logs"
	"github.com/AidarKhusainov/tunwarden/internal/recovery"
	"github.com/AidarKhusainov/tunwarden/internal/status"
)

func TestRunCLIVersion(t *testing.T) {
	var out bytes.Buffer
	if err := run(context.Background(), []string{"version"}, &out); err != nil {
		t.Fatalf("version failed: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "tunwarden") {
		t.Fatalf("expected version output to contain binary name, got %q", got)
	}
}

func TestRunCLIUnknownCommand(t *testing.T) {
	var out bytes.Buffer
	err := run(context.Background(), []string{"unknown"}, &out)
	assertUsageError(t, err, out.String(), "unknown command")
}

func TestRunCLIStatusRendersCleanLocalStatus(t *testing.T) {
	var out bytes.Buffer
	err := runWithOptions(context.Background(), []string{"status"}, &out, options{
		status: func(context.Context) status.Report { return cleanStatusReport() },
	})
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	got := out.String()
	for _, text := range []string{"TunWarden status", "Daemon: not running", "Connection: inactive", "Runtime directory: missing", "Proxy: inactive", "TUN: not managed in this build", "Stale state: none"} {
		if !strings.Contains(got, text) {
			t.Fatalf("expected output to contain %q, got %q", text, got)
		}
	}
}

func TestRunCLIStatusReturnsDiagnosticExitCodeForStaleState(t *testing.T) {
	var out bytes.Buffer
	err := runWithOptions(context.Background(), []string{"status"}, &out, options{
		status: func(context.Context) status.Report {
			report := cleanStatusReport()
			report.Connection = "inactive (stale state detected)"
			report.RuntimeDirectory = status.RuntimeDirectory{Message: "present (stale)"}
			report.Candidates = []status.Candidate{{Kind: "runtime-directory", Description: "runtime directory", Target: "/run/tunwarden"}}
			return report
		},
	})
	if err == nil {
		t.Fatal("expected stale status to return diagnostic exit code")
	}
	if got := ExitCode(err); got != 3 {
		t.Fatalf("expected status diagnostic exit code 3, got %d", got)
	}
	if got := out.String(); !strings.Contains(got, "Guidance: run `tunwarden recover`") {
		t.Fatalf("expected recovery guidance in status output, got %q", got)
	}
}

func TestRunCLIDoctorUsesDaemonWhenAvailable(t *testing.T) {
	var out bytes.Buffer
	err := runWithOptions(context.Background(), []string{"doctor"}, &out, options{
		daemonDoctor: func(context.Context) (doctor.Report, error) {
			return doctor.Report{Source: doctor.SourceDaemon, Checks: []doctor.Check{{Name: "daemon", Severity: doctor.SeverityOK, Message: "running"}}}, nil
		},
	})
	if err != nil {
		t.Fatalf("doctor failed: %v", err)
	}
	got := out.String()
	for _, text := range []string{"TunWarden doctor report", "Source: daemon", "[OK] daemon: running"} {
		if !strings.Contains(got, text) {
			t.Fatalf("expected output to contain %q, got %q", text, got)
		}
	}
}

func TestRunCLIDoctorRejectsUnsupportedArguments(t *testing.T) {
	for _, tt := range []struct {
		name        string
		args        []string
		wantMessage string
	}{
		{name: "json", args: []string{"doctor", "--json"}, wantMessage: "doctor --json is not implemented yet"},
		{name: "core", args: []string{"doctor", "--core"}, wantMessage: "doctor --core is not implemented yet"},
		{name: "garbage", args: []string{"doctor", "garbage"}, wantMessage: "unsupported doctor argument"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := run(context.Background(), tt.args, &out)
			assertUsageError(t, err, out.String(), tt.wantMessage)
		})
	}
}

func TestRunCLILogsHelp(t *testing.T) {
	var out bytes.Buffer
	if err := run(context.Background(), []string{"logs", "--help"}, &out); err != nil {
		t.Fatalf("logs --help failed: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "Usage:\n  tunwarden logs") || !strings.Contains(got, "journalctl") || !strings.Contains(got, "--core") {
		t.Fatalf("expected logs help output, got %q", got)
	}
}

func TestRunCLILogsRunsDefaultDaemonLogs(t *testing.T) {
	var out bytes.Buffer
	var gotOptions logs.Options
	err := runWithOptions(context.Background(), []string{"logs"}, &out, options{
		logs: func(_ context.Context, w io.Writer, opts logs.Options) error {
			gotOptions = opts
			_, _ = fmt.Fprintln(w, "TunWarden daemon logs")
			_, _ = fmt.Fprintln(w, "Jun 03 host tunwardend[123]: daemon started")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("logs failed: %v", err)
	}
	if gotOptions != (logs.Options{}) {
		t.Fatalf("expected default logs options, got %#v", gotOptions)
	}
	if got := out.String(); !strings.Contains(got, "TunWarden daemon logs") || !strings.Contains(got, "daemon started") {
		t.Fatalf("expected daemon logs output, got %q", got)
	}
}

func TestRunCLILogsParsesCoreFollowAndSince(t *testing.T) {
	var out bytes.Buffer
	var gotOptions logs.Options
	err := runWithOptions(context.Background(), []string{"logs", "--core", "--follow", "--since=-30min"}, &out, options{
		logs: func(_ context.Context, w io.Writer, opts logs.Options) error {
			gotOptions = opts
			_, _ = fmt.Fprintln(w, "TunWarden core logs")
			_, _ = fmt.Fprintln(w, "Jun 03 host tunwardend[123]: tunwardend: core xray started pid=42 profile=test-vless")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("logs --core failed: %v", err)
	}
	if !gotOptions.Core || !gotOptions.Follow || gotOptions.Since != "-30min" {
		t.Fatalf("expected core follow since options, got %#v", gotOptions)
	}
	if got := out.String(); !strings.Contains(got, "TunWarden core logs") || !strings.Contains(got, "core xray started") {
		t.Fatalf("expected core logs output, got %q", got)
	}
}

func TestRunCLILogsRejectsUnsupportedArguments(t *testing.T) {
	for _, tt := range []struct {
		name        string
		args        []string
		wantMessage string
	}{
		{name: "json", args: []string{"logs", "--json"}, wantMessage: "logs --json is not implemented yet"},
		{name: "missing-since", args: []string{"logs", "--since"}, wantMessage: "logs --since requires a value"},
		{name: "option-since", args: []string{"logs", "--since", "--follow"}, wantMessage: "logs --since requires a value"},
		{name: "empty-since-equals", args: []string{"logs", "--since="}, wantMessage: "logs --since requires a value"},
		{name: "garbage", args: []string{"logs", "garbage"}, wantMessage: "unsupported logs argument"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := run(context.Background(), tt.args, &out)
			assertUsageError(t, err, out.String(), tt.wantMessage)
		})
	}
}

func TestRunCLIRecoverRendersDryRunPlan(t *testing.T) {
	var out bytes.Buffer
	err := runWithOptions(context.Background(), []string{"recover"}, &out, options{
		recover: func(context.Context) recovery.PlanResult {
			return recovery.PlanResult{Candidates: []recovery.Candidate{{Kind: "tun-interface", Description: "TUN interface", Target: "tunwarden0"}}}
		},
	})
	if err != nil {
		t.Fatalf("recover failed: %v", err)
	}
	got := out.String()
	for _, text := range []string{"TunWarden recovery dry-run", "Would recover TUN interface: tunwarden0", "No changes were applied."} {
		if !strings.Contains(got, text) {
			t.Fatalf("expected output to contain %q, got %q", text, got)
		}
	}
}

func TestRunCLIDoctorReturnsDiagnosticExitCodeForFailures(t *testing.T) {
	var out bytes.Buffer
	err := runWithOptions(context.Background(), []string{"doctor"}, &out, options{
		doctor: func(context.Context) doctor.Report {
			return doctor.Report{Checks: []doctor.Check{{Name: "default-route", Severity: doctor.SeverityFail, Message: "ip route show default failed: test failure"}}}
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

func assertUsageError(t *testing.T, err error, stdout string, wantMessage string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected command to fail")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("expected exit code 2, got %d", got)
	}
	if !strings.Contains(err.Error(), wantMessage) {
		t.Fatalf("expected error containing %q, got %q", wantMessage, err.Error())
	}
	if stdout != "" {
		t.Fatalf("expected no stdout on usage error, got %q", stdout)
	}
}

func cleanStatusReport() status.Report {
	return status.Report{Daemon: "not running", Connection: "inactive", RuntimeDirectory: status.RuntimeDirectory{Message: "missing"}, Proxy: "inactive", TUN: "not managed in this build"}
}

func cleanDoctorReport() doctor.Report {
	return doctor.Report{Source: doctor.SourceLocalFallback, Checks: []doctor.Check{{Name: "platform", Severity: doctor.SeverityOK, Message: "linux/amd64"}}}
}
