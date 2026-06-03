package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/AidarKhusainov/tunwarden/internal/client"
	"github.com/AidarKhusainov/tunwarden/internal/doctor"
	"github.com/AidarKhusainov/tunwarden/internal/logs"
	"github.com/AidarKhusainov/tunwarden/internal/profile"
	"github.com/AidarKhusainov/tunwarden/internal/recovery"
	"github.com/AidarKhusainov/tunwarden/internal/status"
)

const version = "0.0.0-dev"

type exitError struct {
	code int
	err  error
}

func (e exitError) Error() string { return e.err.Error() }
func (e exitError) Unwrap() error { return e.err }

// ExitCode returns the process exit code that should be used for err.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var e exitError
	if errors.As(err, &e) {
		return e.code
	}
	return 1
}

type options struct {
	doctor           func(context.Context) doctor.Report
	daemonDoctor     func(context.Context) (doctor.Report, error)
	logs             func(context.Context, io.Writer, logs.Options) error
	profileStorePath string
	recover          func(context.Context) recovery.PlanResult
	status           func(context.Context) status.Report
	daemonStatus     func(context.Context) (status.Report, error)
}

// Run executes the user-facing TunWarden command line interface.
func Run(ctx context.Context, args []string) error { return run(ctx, args, os.Stdout) }

func run(ctx context.Context, args []string, stdout io.Writer) error {
	return runWithOptions(ctx, args, stdout, options{})
}

func runWithOptions(ctx context.Context, args []string, stdout io.Writer, opts options) error {
	if len(args) == 0 {
		printUsage(stdout)
		return nil
	}
	command := strings.ToLower(args[0])
	commandArgs := args[1:]
	switch command {
	case "help":
		return runHelp(commandArgs, stdout)
	case "-h", "--help":
		if len(commandArgs) > 0 {
			return usageError("%s does not accept arguments", args[0])
		}
		printUsage(stdout)
		return nil
	case "version", "--version":
		return runVersionCommand(commandArgs, stdout)
	case "profile":
		return runProfileCommand(ctx, commandArgs, stdout, opts)
	case "status":
		return runStatusCommand(ctx, commandArgs, stdout, opts)
	case "doctor":
		return runDoctorCommand(ctx, commandArgs, stdout, opts)
	case "logs":
		return runLogsCommand(ctx, commandArgs, stdout, opts)
	case "recover":
		return runRecoverCommand(ctx, commandArgs, stdout, opts)
	default:
		return usageError("unknown command %q", args[0])
	}
}

func runHelp(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		printUsage(stdout)
		return nil
	}
	if len(args) > 1 {
		return usageError("help accepts at most one command")
	}
	switch strings.ToLower(args[0]) {
	case "version":
		printVersionHelp(stdout)
	case "profile":
		printProfileHelp(stdout)
	case "status":
		printStatusHelp(stdout)
	case "doctor":
		printDoctorHelp(stdout)
	case "logs":
		printLogsHelp(stdout)
	case "recover":
		printRecoverHelp(stdout)
	default:
		return usageError("unknown help topic %q", args[0])
	}
	return nil
}

func runVersionCommand(args []string, stdout io.Writer) error {
	if isHelp(args) {
		printVersionHelp(stdout)
		return nil
	}
	if len(args) > 0 {
		return usageError("version does not accept arguments")
	}
	fmt.Fprintf(stdout, "tunwarden %s\n", version)
	return nil
}

func runProfileCommand(ctx context.Context, args []string, stdout io.Writer, opts options) error {
	_ = ctx
	if isHelp(args) {
		printProfileHelp(stdout)
		return nil
	}
	if len(args) == 0 {
		return usageError("profile requires a subcommand")
	}
	store, err := profile.NewStore(opts.profileStorePath)
	if err != nil {
		return err
	}
	switch strings.ToLower(args[0]) {
	case "add":
		return runProfileAdd(store, args[1:], stdout)
	case "list":
		return runProfileList(store, args[1:], stdout)
	case "show":
		return runProfileShow(store, args[1:], stdout)
	case "delete":
		return runProfileDelete(store, args[1:], stdout)
	default:
		return usageError("unknown profile subcommand %q", args[0])
	}
}

func runProfileAdd(store profile.Store, args []string, stdout io.Writer) error {
	parsed, err := parseProfileAddArgs(args)
	if err != nil {
		return err
	}
	p := profile.NewManual(parsed.name, parsed.server, parsed.port, parsed.protocol)
	if err := store.Add(p); err != nil {
		return profileCommandError(err)
	}
	fmt.Fprintf(stdout, "Profile added: %s\n", p.ID)
	return nil
}

func runProfileList(store profile.Store, args []string, stdout io.Writer) error {
	jsonOutput, err := parseOptionalJSON(args, "profile list")
	if err != nil {
		return err
	}
	profiles, err := store.List()
	if err != nil {
		return err
	}
	if jsonOutput {
		return writeJSON(stdout, map[string]any{"schema_version": "v1", "profiles": profiles})
	}
	fmt.Fprintln(stdout, "ID        NAME   PROTOCOL  SERVER       PORT")
	for _, p := range profiles {
		fmt.Fprintf(stdout, "%-9s %-6s %-9s %-12s %d\n", p.ID, p.Name, p.Protocol, p.Server, p.Port)
	}
	return nil
}

func runProfileShow(store profile.Store, args []string, stdout io.Writer) error {
	id, jsonOutput, err := parseProfileShowArgs(args)
	if err != nil {
		return err
	}
	p, err := store.Get(id)
	if err != nil {
		return profileCommandError(err)
	}
	if jsonOutput {
		return writeJSON(stdout, map[string]any{"schema_version": "v1", "profile": p})
	}
	fmt.Fprintf(stdout, "ID: %s\n", p.ID)
	fmt.Fprintf(stdout, "Name: %s\n", p.Name)
	fmt.Fprintf(stdout, "Source: %s\n", p.Source)
	fmt.Fprintf(stdout, "Engine: %s\n", p.Engine)
	fmt.Fprintf(stdout, "Protocol: %s\n", p.Protocol)
	fmt.Fprintf(stdout, "Server: %s\n", p.Server)
	fmt.Fprintf(stdout, "Port: %d\n", p.Port)
	return nil
}

func runProfileDelete(store profile.Store, args []string, stdout io.Writer) error {
	id, yes, err := parseProfileDeleteArgs(args)
	if err != nil {
		return err
	}
	if !yes {
		return usageError("profile delete requires --yes in this non-interactive v0.1 CLI")
	}
	if err := store.Delete(id); err != nil {
		return profileCommandError(err)
	}
	fmt.Fprintf(stdout, "Profile deleted: %s\n", id)
	return nil
}

type profileAddArgs struct {
	name     string
	server   string
	port     uint16
	protocol string
}

func parseProfileAddArgs(args []string) (profileAddArgs, error) {
	var parsed profileAddArgs
	for i := 0; i < len(args); i++ {
		arg := args[i]
		value, hasInlineValue := cutFlagValue(arg)
		switch {
		case arg == "--name" || strings.HasPrefix(arg, "--name="):
			v, next, err := flagValue("profile add --name", args, i, value, hasInlineValue)
			if err != nil {
				return parsed, err
			}
			parsed.name = v
			i = next
		case arg == "--server" || strings.HasPrefix(arg, "--server="):
			v, next, err := flagValue("profile add --server", args, i, value, hasInlineValue)
			if err != nil {
				return parsed, err
			}
			parsed.server = v
			i = next
		case arg == "--protocol" || strings.HasPrefix(arg, "--protocol="):
			v, next, err := flagValue("profile add --protocol", args, i, value, hasInlineValue)
			if err != nil {
				return parsed, err
			}
			parsed.protocol = v
			i = next
		case arg == "--port" || strings.HasPrefix(arg, "--port="):
			v, next, err := flagValue("profile add --port", args, i, value, hasInlineValue)
			if err != nil {
				return parsed, err
			}
			port, err := strconv.ParseUint(v, 10, 16)
			if err != nil || port == 0 {
				return parsed, usageError("profile add --port must be a number between 1 and 65535")
			}
			parsed.port = uint16(port)
			i = next
		case arg == "--json":
			return parsed, usageError("profile add --json is not implemented")
		default:
			return parsed, usageError("unsupported profile add argument %q", arg)
		}
	}
	p := profile.NewManual(parsed.name, parsed.server, parsed.port, parsed.protocol)
	if err := profile.Validate(p); err != nil {
		return parsed, usageError(err.Error())
	}
	return parsed, nil
}

func parseProfileShowArgs(args []string) (string, bool, error) {
	var id string
	var jsonOutput bool
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOutput = true
		default:
			if strings.HasPrefix(arg, "-") {
				return "", false, usageError("unsupported profile show argument %q", arg)
			}
			if id != "" {
				return "", false, usageError("profile show accepts exactly one profile id")
			}
			id = arg
		}
	}
	if id == "" {
		return "", false, usageError("profile show requires a profile id")
	}
	return id, jsonOutput, nil
}

func parseProfileDeleteArgs(args []string) (string, bool, error) {
	var id string
	var yes bool
	for _, arg := range args {
		switch arg {
		case "--yes":
			yes = true
		case "--json":
			return "", false, usageError("profile delete --json is not implemented")
		default:
			if strings.HasPrefix(arg, "-") {
				return "", false, usageError("unsupported profile delete argument %q", arg)
			}
			if id != "" {
				return "", false, usageError("profile delete accepts exactly one profile id")
			}
			id = arg
		}
	}
	if id == "" {
		return "", false, usageError("profile delete requires a profile id")
	}
	return id, yes, nil
}

func parseOptionalJSON(args []string, command string) (bool, error) {
	var jsonOutput bool
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
			continue
		}
		return false, usageError("unsupported %s argument %q", command, arg)
	}
	return jsonOutput, nil
}

func cutFlagValue(arg string) (string, bool) {
	_, value, ok := strings.Cut(arg, "=")
	return value, ok
}

func flagValue(flag string, args []string, index int, inlineValue string, hasInlineValue bool) (string, int, error) {
	if hasInlineValue {
		if strings.TrimSpace(inlineValue) == "" {
			return "", index, usageError("%s requires a value", flag)
		}
		return inlineValue, index, nil
	}
	if index+1 >= len(args) || strings.TrimSpace(args[index+1]) == "" || strings.HasPrefix(args[index+1], "--") {
		return "", index, usageError("%s requires a value", flag)
	}
	return args[index+1], index + 1, nil
}

func writeJSON(stdout io.Writer, value any) error {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func profileCommandError(err error) error {
	switch {
	case errors.Is(err, profile.ErrNotFound):
		return exitError{code: 1, err: err}
	case errors.Is(err, profile.ErrAlreadyExists):
		return exitError{code: 1, err: err}
	case profile.IsValidationError(err):
		return usageError(err.Error())
	default:
		return err
	}
}

func runStatusCommand(ctx context.Context, args []string, stdout io.Writer, opts options) error {
	if isHelp(args) {
		printStatusHelp(stdout)
		return nil
	}
	if len(args) > 0 {
		return unsupportedStatusArgument(args[0])
	}
	report := runStatus(ctx, opts)
	fmt.Fprint(stdout, report.String())
	if report.HasUnhealthyState() {
		return exitError{code: 3, err: errors.New("status found stale or incomplete local state")}
	}
	return nil
}

func runDoctorCommand(ctx context.Context, args []string, stdout io.Writer, opts options) error {
	if isHelp(args) {
		printDoctorHelp(stdout)
		return nil
	}
	if len(args) > 0 {
		return unsupportedDoctorArgument(args[0])
	}
	report := runDoctor(ctx, opts)
	fmt.Fprint(stdout, report.String())
	if report.HasFailures() {
		return exitError{code: 3, err: errors.New("doctor found failing checks")}
	}
	return nil
}

func runLogsCommand(ctx context.Context, args []string, stdout io.Writer, opts options) error {
	if isHelp(args) {
		printLogsHelp(stdout)
		return nil
	}
	logOptions, err := parseLogsArgs(args)
	if err != nil {
		return err
	}
	return runLogs(ctx, stdout, opts, logOptions)
}

func runRecoverCommand(ctx context.Context, args []string, stdout io.Writer, opts options) error {
	if isHelp(args) {
		printRecoverHelp(stdout)
		return nil
	}
	if len(args) > 0 {
		if contains(args, "--execute") {
			return usageError("recover --execute is not implemented in v0.1")
		}
		return usageError("unsupported recover argument %q", args[0])
	}
	plan := runRecover(ctx, opts)
	fmt.Fprint(stdout, plan.String())
	return nil
}

func unsupportedStatusArgument(arg string) error {
	switch arg {
	case "--json":
		return usageError("status --json is not implemented yet")
	default:
		return usageError("unsupported status argument %q", arg)
	}
}

func unsupportedDoctorArgument(arg string) error {
	switch arg {
	case "--json":
		return usageError("doctor --json is not implemented yet")
	case "--core", "--network", "--dns", "--routes", "--firewall":
		return usageError("doctor %s is not implemented yet", arg)
	default:
		return usageError("unsupported doctor argument %q", arg)
	}
}

func parseLogsArgs(args []string) (logs.Options, error) {
	var opts logs.Options
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if value, ok := strings.CutPrefix(arg, "--since="); ok {
			if strings.TrimSpace(value) == "" {
				return opts, usageError("logs --since requires a value")
			}
			opts.Since = value
			continue
		}
		switch arg {
		case "--follow", "-f":
			opts.Follow = true
		case "--daemon":
		case "--since":
			i++
			if i >= len(args) || strings.TrimSpace(args[i]) == "" || isLogsOption(args[i]) {
				return opts, usageError("logs --since requires a value")
			}
			opts.Since = args[i]
		case "--json":
			return opts, usageError("logs --json is not implemented yet")
		case "--core":
			return opts, usageError("logs --core is not implemented yet")
		default:
			return opts, usageError("unsupported logs argument %q", arg)
		}
	}
	return opts, nil
}

func isLogsOption(arg string) bool {
	if _, ok := strings.CutPrefix(arg, "--since="); ok {
		return true
	}
	switch arg {
	case "--follow", "-f", "--daemon", "--since", "--json", "--core":
		return true
	default:
		return false
	}
}

func runStatus(ctx context.Context, opts options) status.Report {
	if opts.status != nil {
		return opts.status(ctx)
	}
	daemonStatus, err := runDaemonStatus(ctx, opts)
	if err == nil {
		return daemonStatus
	}
	local := status.Inspect(ctx)
	if client.IsDaemonUnavailable(err) {
		return status.WithDaemonUnavailable(local, client.UnavailableMessage(err))
	}
	local.Warnings = append(local.Warnings, status.Warning{Target: "daemon status API", Message: err.Error()})
	if local.Connection == "inactive" {
		local.Connection = "unknown (inspection incomplete)"
	}
	return local
}

func runDaemonStatus(ctx context.Context, opts options) (status.Report, error) {
	if opts.daemonStatus != nil {
		return opts.daemonStatus(ctx)
	}
	response, err := (client.StatusClient{}).Status(ctx)
	if err != nil {
		return status.Report{}, err
	}
	return status.FromDaemon(response), nil
}

func runDoctor(ctx context.Context, opts options) doctor.Report {
	if opts.daemonDoctor != nil {
		report, err := opts.daemonDoctor(ctx)
		if err == nil {
			return report
		}
		return localDoctorWithDaemonWarning(ctx, opts, err)
	}
	if opts.doctor != nil {
		return opts.doctor(ctx)
	}
	daemonDoctor, err := runDaemonDoctor(ctx, opts)
	if err == nil {
		return daemonDoctor
	}
	return localDoctorWithDaemonWarning(ctx, opts, err)
}

func runDaemonDoctor(ctx context.Context, opts options) (doctor.Report, error) {
	if opts.daemonDoctor != nil {
		return opts.daemonDoctor(ctx)
	}
	response, err := (client.DoctorClient{}).Doctor(ctx)
	if err != nil {
		return doctor.Report{}, err
	}
	return doctor.FromDaemon(response), nil
}

func localDoctorWithDaemonWarning(ctx context.Context, opts options, err error) doctor.Report {
	local := localDoctor(ctx, opts)
	message := err.Error()
	if client.IsDaemonUnavailable(err) {
		message = client.UnavailableMessage(err)
	} else {
		message = "daemon doctor API unavailable: " + message
	}
	return doctor.WithDaemonCheck(local, doctor.SeverityWarning, message)
}

func localDoctor(ctx context.Context, opts options) doctor.Report {
	if opts.doctor != nil {
		return opts.doctor(ctx)
	}
	return doctor.Run(ctx)
}

func runLogs(ctx context.Context, stdout io.Writer, opts options, logOptions logs.Options) error {
	if opts.logs != nil {
		return opts.logs(ctx, stdout, logOptions)
	}
	return logs.Run(ctx, stdout, logOptions)
}

func runRecover(ctx context.Context, opts options) recovery.PlanResult {
	if opts.recover != nil {
		return opts.recover(ctx)
	}
	return recovery.Plan(ctx)
}

func isHelp(args []string) bool { return len(args) == 1 && (args[0] == "-h" || args[0] == "--help") }

func contains(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func usageError(format string, args ...any) error { return exitError{code: 2, err: fmt.Errorf(format, args...)} }

func printUsage(w io.Writer) {
	fmt.Fprint(w, `TunWarden - Linux-first safe TUN VPN client foundation

Usage:
  tunwarden version
  tunwarden profile <add|list|show|delete>
  tunwarden status
  tunwarden doctor
  tunwarden logs
  tunwarden recover
  tunwarden help [command]

Current status:
  This is an early foundation build. Commands manage local profiles, print
  daemon-backed or local status, diagnostics, daemon logs, and recovery plans;
  they do not yet mutate system networking state.
`)
}

func printVersionHelp(w io.Writer) {
	fmt.Fprint(w, `Usage:
  tunwarden version

Print the TunWarden CLI version.
`)
}

func printProfileHelp(w io.Writer) {
	fmt.Fprint(w, `Usage:
  tunwarden profile add --name <name> --server <host> --port <port> --protocol <vless|vmess|trojan|shadowsocks>
  tunwarden profile list [--json]
  tunwarden profile show <profile-id> [--json]
  tunwarden profile delete <profile-id> --yes

Manage manual profiles in local TunWarden user state. These commands never start
network processes and never mutate TUN, routes, DNS, nftables, or firewall state.

Implemented in v0.1:
  manual profile add, list, show, delete, validation, JSON list/show output, and
  atomic local profile storage under the documented XDG user state location.

Not implemented yet:
  profile import, VLESS URI import, subscriptions, Xray config generation,
  connect/disconnect behavior
`)
}

func printStatusHelp(w io.Writer) {
	fmt.Fprint(w, `Usage:
  tunwarden status

Report local TunWarden runtime state. The command uses daemon-backed status
when the local Unix socket API is reachable and falls back to read-only local
inspection when it is not.

Implemented in v0.1:
  daemon-backed inactive status, conservative local fallback, runtime directory
  state, stale runtime candidate summary, and recovery guidance.

Not implemented yet:
  --json, active profile/mode, proxy process lifecycle, core health
`)
}

func printDoctorHelp(w io.Writer) {
	fmt.Fprint(w, `Usage:
  tunwarden doctor

Run read-only diagnostics for the current Linux host. The command uses
daemon-backed diagnostics when the local Unix socket API is reachable and falls
back to local read-only diagnostics when it is not.

Implemented in v0.1:
  daemon-backed source reporting, local fallback, platform, command
  availability, default route, default interface, and stale TunWarden-owned
  resource detection.

Not implemented yet:
  --json, --core, --network, --dns, --routes, --firewall
`)
}

func printLogsHelp(w io.Writer) {
	fmt.Fprint(w, `Usage:
  tunwarden logs [--follow] [--daemon] [--since <duration>]
  tunwarden logs -f

Print recent tunwardend logs from the system journal using journalctl. This
command is read-only and applies the standard TunWarden output redaction policy
before printing log lines.

Implemented in v0.1:
  recent daemon logs, --follow, -f, --daemon, --since

Not implemented yet:
  --json, --core
`)
}

func printRecoverHelp(w io.Writer) {
	fmt.Fprint(w, `Usage:
  tunwarden recover

Print the read-only recovery dry-run plan for clearly TunWarden-owned resources.
Cleanup execution is intentionally not implemented in v0.1; recover --execute is
rejected.
`)
}
