package status

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AidarKhusainov/tunwarden/internal/api"
	"github.com/AidarKhusainov/tunwarden/internal/render"
	txstate "github.com/AidarKhusainov/tunwarden/internal/state"
)

const generatedDirName = "generated"

type RuntimeDirectoryState string

const (
	RuntimeDirectoryMissing RuntimeDirectoryState = "missing"
	RuntimeDirectoryPresent RuntimeDirectoryState = "present"
	RuntimeDirectoryPath    RuntimeDirectoryState = "path"
	RuntimeDirectoryUnknown RuntimeDirectoryState = "unknown"
)

type RuntimeDirectory struct {
	Path    string
	State   RuntimeDirectoryState
	Message string
}

type Candidate struct {
	Kind        string
	Description string
	Target      string
}

type Warning struct {
	Target  string
	Message string
}

type Report struct {
	Daemon            string
	Service           string
	Connection        string
	Mode              string
	RuntimeDirectory  RuntimeDirectory
	RuntimeConfigPath string
	Proxy             string
	TUN               string
	Routes            string
	DNS               string
	Firewall          string
	Transactions      []txstate.TransactionSummary
	Candidates        []Candidate
	Warnings          []Warning
}

type Options struct {
	RuntimeDir string
}

func Inspect(ctx context.Context) Report {
	return InspectWithOptions(ctx, Options{})
}

func InspectWithOptions(ctx context.Context, opts Options) Report {
	runtimeDir := opts.RuntimeDir
	if runtimeDir == "" {
		runtimeDir = api.RuntimeDirFromEnv()
	}

	report := Report{
		Daemon:  "not running",
		Service: "none",
		RuntimeDirectory: RuntimeDirectory{
			Path: runtimeDir,
		},
		Proxy: "inactive",
		TUN:   "not managed in this build",
	}

	select {
	case <-ctx.Done():
		report.Connection = "unknown (inspection incomplete)"
		report.RuntimeDirectory.State = RuntimeDirectoryUnknown
		report.RuntimeDirectory.Message = "unknown (inspection incomplete)"
		report.Warnings = append(report.Warnings, Warning{
			Target:  "runtime directory " + runtimeDir,
			Message: ctx.Err().Error(),
		})
		return report
	default:
	}

	runtime, runtimeWarning := inspectRuntimeDirectory(runtimeDir)
	report.RuntimeDirectory = runtime
	if runtimeWarning != nil {
		report.Warnings = append(report.Warnings, *runtimeWarning)
	}

	generated, generatedWarning := inspectGeneratedRuntimeConfigs(filepath.Join(runtimeDir, generatedDirName))
	report.Candidates = append(report.Candidates, generated...)
	if generatedWarning != nil {
		report.Warnings = append(report.Warnings, *generatedWarning)
	}

	transactions, transactionCandidates, transactionWarnings := inspectTransactions(runtimeDir)
	report.Transactions = transactions
	report.Candidates = append(report.Candidates, transactionCandidates...)
	report.Warnings = append(report.Warnings, transactionWarnings...)
	report.Candidates = append(report.Candidates, runtimeCandidate(runtime)...)
	report.Connection = connectionState(report.Candidates, report.Warnings)
	return report
}

func FromDaemon(s api.StatusResponse) Report {
	return Report{
		Daemon:     s.Daemon,
		Service:    s.Service,
		Connection: s.Connection,
		Mode:       s.Mode,
		RuntimeDirectory: RuntimeDirectory{
			State:   RuntimeDirectoryPresent,
			Message: s.RuntimeDirectory,
		},
		RuntimeConfigPath: s.RuntimeConfigPath,
		Proxy:             s.Proxy,
		TUN:               s.TUN,
		Routes:            s.Routes,
		DNS:               s.DNS,
		Firewall:          s.Firewall,
		Transactions:      transactionsFromAPI(s.Transactions),
		Warnings:          warningsFromStrings("daemon", s.Warnings),
	}
}

func WithDaemonUnavailable(base Report, message string) Report {
	base.Daemon = "not reachable (" + message + "); using local fallback"
	return base
}

func (r Report) HasUnhealthyState() bool {
	if len(r.Candidates) > 0 || len(r.Warnings) > 0 {
		return true
	}
	for _, tx := range r.Transactions {
		if tx.RequiresCleanup {
			return true
		}
	}
	return false
}

func (r Report) String() string {
	var b strings.Builder
	b.WriteString("TunWarden status\n")
	fmt.Fprintf(&b, "Daemon: %s\n", render.Redact(r.Daemon))
	fmt.Fprintf(&b, "Service: %s\n", render.Redact(serviceLine(r.Service)))
	fmt.Fprintf(&b, "Connection: %s\n", render.Redact(r.Connection))
	if r.Mode != "" {
		fmt.Fprintf(&b, "Mode: %s\n", render.Redact(r.Mode))
	}
	fmt.Fprintf(&b, "Runtime directory: %s\n", render.Redact(r.RuntimeDirectory.Message))
	if r.RuntimeConfigPath != "" {
		fmt.Fprintf(&b, "Runtime config: %s\n", render.Redact(r.RuntimeConfigPath))
	}
	fmt.Fprintf(&b, "Proxy: %s\n", render.Redact(r.Proxy))
	fmt.Fprintf(&b, "TUN: %s\n", render.Redact(r.TUN))
	if r.Routes != "" {
		fmt.Fprintf(&b, "Routes: %s\n", render.Redact(r.Routes))
	}
	if r.DNS != "" {
		fmt.Fprintf(&b, "DNS: %s\n", render.Redact(r.DNS))
	}
	if r.Firewall != "" {
		fmt.Fprintf(&b, "Firewall: %s\n", render.Redact(r.Firewall))
	}
	fmt.Fprintf(&b, "Stale state: %s\n", render.Redact(staleStateLine(r.Candidates, r.Warnings)))
	for _, tx := range r.Transactions {
		fmt.Fprintf(&b, "Transaction: %s\n", render.Redact(tx.StatusLine()))
		fmt.Fprintf(&b, "Rollback available: %s\n", render.Redact(tx.RollbackLine()))
		fmt.Fprintf(&b, "State path: %s\n", render.Redact(tx.Path))
	}
	if len(r.Candidates) > 0 {
		b.WriteString("Recovery candidates:\n")
		for _, c := range r.Candidates {
			fmt.Fprintf(&b, "  - %s: %s\n", render.Redact(c.Description), render.Redact(c.Target))
		}
	}
	if len(r.Warnings) > 0 {
		b.WriteString("Inspection warnings:\n")
		for _, w := range r.Warnings {
			fmt.Fprintf(&b, "  - could not inspect %s: %s\n", render.Redact(w.Target), render.Redact(w.Message))
		}
	}
	switch {
	case len(r.Candidates) > 0 && len(r.Warnings) > 0:
		b.WriteString("Guidance: run `tunwarden recover` for the canonical read-only recovery dry-run and `tunwarden doctor` for diagnostic detail.\n")
	case len(r.Candidates) > 0:
		b.WriteString("Guidance: run `tunwarden recover` for the canonical read-only recovery dry-run.\n")
	case len(r.Warnings) > 0:
		b.WriteString("Guidance: run `tunwarden doctor` for diagnostic detail.\n")
	}
	return b.String()
}

func inspectRuntimeDirectory(runtimeDir string) (RuntimeDirectory, *Warning) {
	runtime := RuntimeDirectory{Path: runtimeDir}
	stat, err := os.Stat(runtimeDir)
	switch {
	case err == nil && stat.IsDir():
		runtime.State = RuntimeDirectoryPresent
		runtime.Message = "present (stale)"
		return runtime, nil
	case err == nil:
		runtime.State = RuntimeDirectoryPath
		runtime.Message = "present as non-directory (stale)"
		return runtime, nil
	case errors.Is(err, os.ErrNotExist):
		runtime.State = RuntimeDirectoryMissing
		runtime.Message = "missing"
		return runtime, nil
	default:
		runtime.State = RuntimeDirectoryUnknown
		runtime.Message = "unknown (inspection incomplete)"
		return runtime, &Warning{Target: "runtime directory " + runtimeDir, Message: err.Error()}
	}
}

func inspectGeneratedRuntimeConfigs(generatedDir string) ([]Candidate, *Warning) {
	stat, err := os.Stat(generatedDir)
	switch {
	case err == nil && stat.IsDir():
		return []Candidate{{Kind: "generated-runtime-configs", Description: "generated runtime configs", Target: generatedDir}}, nil
	case err == nil:
		return []Candidate{{Kind: "generated-runtime-configs", Description: "generated runtime config path", Target: generatedDir}}, nil
	case errors.Is(err, os.ErrNotExist):
		return nil, nil
	default:
		return nil, &Warning{Target: "generated runtime configs " + generatedDir, Message: err.Error()}
	}
}

func inspectTransactions(runtimeDir string) ([]txstate.TransactionSummary, []Candidate, []Warning) {
	summaries, scanWarnings := txstate.ScanTransactions(runtimeDir)
	candidates := make([]Candidate, 0, len(summaries))
	for _, summary := range summaries {
		if summary.RequiresCleanup {
			candidates = append(candidates, Candidate{Kind: "transaction-state", Description: "transaction rollback state", Target: summary.Path})
		}
	}
	return summaries, candidates, warningsFromStrings("transaction state", scanWarnings)
}

func runtimeCandidate(runtime RuntimeDirectory) []Candidate {
	switch runtime.State {
	case RuntimeDirectoryPresent:
		return []Candidate{{Kind: "runtime-directory", Description: "runtime directory", Target: runtime.Path}}
	case RuntimeDirectoryPath:
		return []Candidate{{Kind: "runtime-directory", Description: "runtime path", Target: runtime.Path}}
	default:
		return nil
	}
}

func connectionState(candidates []Candidate, warnings []Warning) string {
	if len(candidates) > 0 {
		return "inactive (stale state detected)"
	}
	if len(warnings) > 0 {
		return "unknown (inspection incomplete)"
	}
	return "inactive"
}

func serviceLine(service string) string {
	if service == "" {
		return "unknown"
	}
	return service
}

func staleStateLine(candidates []Candidate, warnings []Warning) string {
	switch {
	case len(candidates) > 0 && len(warnings) > 0:
		return fmt.Sprintf("found %d recovery %s; inspection incomplete", len(candidates), candidateNoun(len(candidates)))
	case len(candidates) > 0:
		return fmt.Sprintf("found %d recovery %s", len(candidates), candidateNoun(len(candidates)))
	case len(warnings) > 0:
		return "unknown (inspection incomplete)"
	default:
		return "none"
	}
}

func candidateNoun(count int) string {
	if count == 1 {
		return "candidate"
	}
	return "candidates"
}

func warningsFromStrings(target string, warnings []string) []Warning {
	out := make([]Warning, 0, len(warnings))
	for _, warning := range warnings {
		out = append(out, Warning{Target: target, Message: warning})
	}
	return out
}

func transactionsFromAPI(in []api.TransactionStatus) []txstate.TransactionSummary {
	out := make([]txstate.TransactionSummary, 0, len(in))
	for _, tx := range in {
		out = append(out, txstate.TransactionSummary{
			ID:                tx.ID,
			State:             txstate.TransactionState(tx.State),
			Path:              tx.Path,
			RollbackAvailable: tx.RollbackAvailable,
			RequiresCleanup:   tx.RequiresCleanup,
		})
	}
	return out
}
