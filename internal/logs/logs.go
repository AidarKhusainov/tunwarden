package logs

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/AidarKhusainov/tunwarden/internal/render"
)

const (
	DaemonUnit   = "tunwardend.service"
	DefaultLines = "200"
)

// Options describes the read-only log stream requested by the CLI.
type Options struct {
	Follow bool
	Since  string
}

// Run prints recent tunwardend logs from journald.
func Run(ctx context.Context, stdout io.Writer, opts Options) error {
	if _, err := exec.LookPath("journalctl"); err != nil {
		return errors.New("journalctl is not available; install systemd journal tools or run on a systemd/journald host")
	}

	fmt.Fprintln(stdout, "TunWarden daemon logs")
	return RunJournalctl(ctx, stdout, opts)
}

// BuildJournalctlArgs returns the exact journalctl argument vector for daemon logs.
func BuildJournalctlArgs(opts Options) []string {
	args := []string{
		"--unit", DaemonUnit,
		"--no-pager",
		"--output", "short",
	}
	if opts.Since != "" {
		args = append(args, "--since", opts.Since)
	} else {
		args = append(args, "--lines", DefaultLines)
	}
	if opts.Follow {
		args = append(args, "--follow")
	}
	return args
}

// RunJournalctl executes journalctl and renders redacted output lines.
func RunJournalctl(ctx context.Context, stdout io.Writer, opts Options) error {
	cmd := exec.CommandContext(ctx, "journalctl", BuildJournalctlArgs(opts)...)

	outPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("prepare journalctl stdout: %w", err)
	}
	errPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("prepare journalctl stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start journalctl for %s: %w", DaemonUnit, err)
	}

	var stderr bytes.Buffer
	stdoutErr := scanRedacted(stdout, outPipe)
	stderrErr := scanRedacted(&stderr, errPipe)
	waitErr := cmd.Wait()

	if stdoutErr != nil {
		return fmt.Errorf("read journalctl output: %w", stdoutErr)
	}
	if stderrErr != nil {
		return fmt.Errorf("read journalctl error output: %w", stderrErr)
	}
	if waitErr != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = waitErr.Error()
		}
		return fmt.Errorf("journalctl failed for %s: %s", DaemonUnit, render.Redact(message))
	}
	return nil
}

func scanRedacted(dst io.Writer, src io.Reader) error {
	scanner := bufio.NewScanner(src)
	for scanner.Scan() {
		fmt.Fprintln(dst, render.Redact(scanner.Text()))
	}
	return scanner.Err()
}
