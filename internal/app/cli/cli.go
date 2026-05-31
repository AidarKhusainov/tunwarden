package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/AidarKhusainov/tunwarden/internal/doctor"
	"github.com/AidarKhusainov/tunwarden/internal/recovery"
)

const version = "0.0.0-dev"

type exitError struct {
	code int
	err  error
}

func (e exitError) Error() string {
	return e.err.Error()
}

func (e exitError) Unwrap() error {
	return e.err
}

// ExitCode returns the process exit code that should be used for err.
func ExitCode(err error) int {
	var e exitError
	if errors.As(err, &e) {
		return e.code
	}
	return 1
}

type options struct {
	doctor  func(context.Context) doctor.Report
	recover func() recovery.PlanResult
}

// Run executes the user-facing TunWarden command line interface.
func Run(ctx context.Context, args []string) error {
	return run(ctx, args, os.Stdout)
}

func run(ctx context.Context, args []string, stdout io.Writer) error {
	return runWithOptions(ctx, args, stdout, options{})
}

func runWithOptions(ctx context.Context, args []string, stdout io.Writer, opts options) error {
	if len(args) == 0 {
		printUsage(stdout)
		return nil
	}

	switch strings.ToLower(args[0]) {
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	case "version", "--version":
		fmt.Fprintf(stdout, "tunwarden %s\n", version)
		return nil
	case "doctor":
		report := runDoctor(ctx, opts)
		fmt.Fprint(stdout, report.String())
		if report.HasFailures() {
			return exitError{code: 3, err: errors.New("doctor found failing checks")}
		}
		return nil
	case "recover":
		plan := runRecover(opts)
		fmt.Fprint(stdout, plan.String())
		return nil
	default:
		return exitError{code: 2, err: fmt.Errorf("unknown command %q", args[0])}
	}
}

func runDoctor(ctx context.Context, opts options) doctor.Report {
	if opts.doctor != nil {
		return opts.doctor(ctx)
	}
	return doctor.Run(ctx)
}

func runRecover(opts options) recovery.PlanResult {
	if opts.recover != nil {
		return opts.recover()
	}
	return recovery.Plan()
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `TunWarden - Linux-first safe TUN VPN client foundation

Usage:
  tunwarden version
  tunwarden doctor
  tunwarden recover

Current status:
  This is an early foundation build. Commands print contracts and diagnostic
  plans; they do not yet mutate system networking state.
`)
}
