package daemon

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const (
	// PolkitAuthorizationEnv controls optional daemon-side polkit checks for
	// privileged lifecycle and recovery operations. Unset keeps the socket-group
	// access model as the fallback.
	PolkitAuthorizationEnv = "TUNWARDEN_POLKIT_AUTHORIZATION"

	polkitActionPrefix = "io.github.aidarkhusainov.tunwarden."
)

const (
	ActionConnectProxyOnly AuthorizationAction = polkitActionPrefix + "connect-proxy-only"
	ActionConnectTun       AuthorizationAction = polkitActionPrefix + "connect-tun"
	ActionDisconnect       AuthorizationAction = polkitActionPrefix + "disconnect"
	ActionRecoverExecute   AuthorizationAction = polkitActionPrefix + "recover-execute"
)

var (
	ErrAuthorizationDenied      = errors.New("authorization denied")
	ErrAuthorizationUnavailable = errors.New("authorization unavailable")
)

type AuthorizationAction string

type PeerSubject struct {
	PID int
	UID uint32
	GID uint32
}

type Authorizer interface {
	Authorize(context.Context, AuthorizationAction, PeerSubject) error
}

type peerCredentialRequirement interface {
	RequiresPeerCredentials() bool
}

type AllowAuthorizer struct{}

func (AllowAuthorizer) Authorize(context.Context, AuthorizationAction, PeerSubject) error {
	return nil
}

func (AllowAuthorizer) RequiresPeerCredentials() bool {
	return false
}

type StaticErrorAuthorizer struct {
	Err error
}

func (a StaticErrorAuthorizer) Authorize(context.Context, AuthorizationAction, PeerSubject) error {
	if a.Err == nil {
		return nil
	}
	return a.Err
}

func (StaticErrorAuthorizer) RequiresPeerCredentials() bool {
	return false
}

type PolkitAuthorizer struct {
	CommandPath          string
	AllowUserInteraction bool

	lookupPath func(string) (string, error)
	run        func(context.Context, string, []string) error
}

func (PolkitAuthorizer) RequiresPeerCredentials() bool {
	return true
}

func (a PolkitAuthorizer) Authorize(ctx context.Context, action AuthorizationAction, subject PeerSubject) error {
	if action == "" {
		return fmt.Errorf("%w: missing polkit action", ErrAuthorizationUnavailable)
	}
	if subject.PID <= 0 {
		return fmt.Errorf("%w: missing local peer process for %s", ErrAuthorizationUnavailable, action)
	}

	command, err := a.resolveCommand()
	if err != nil {
		return err
	}
	subjectSpec := strconv.Itoa(subject.PID)
	args := []string{"--action-id", string(action), "--process", subjectSpec}
	if a.AllowUserInteraction {
		args = append(args, "--allow-user-interaction")
	}
	if err := a.runCommand(ctx, command, args); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return fmt.Errorf("%w: polkit denied %s; keep using the non-root tunwarden CLI and authenticate through a desktop or TTY polkit agent when available", ErrAuthorizationDenied, action)
		}
		return fmt.Errorf("%w: polkit could not authorize %s; ensure a polkit authentication agent is available or disable %s to use socket-group fallback", ErrAuthorizationUnavailable, action, PolkitAuthorizationEnv)
	}
	return nil
}

func (a PolkitAuthorizer) resolveCommand() (string, error) {
	command := strings.TrimSpace(a.CommandPath)
	if command == "" {
		command = "pkcheck"
	}
	if strings.ContainsRune(command, os.PathSeparator) {
		return command, nil
	}
	lookupPath := exec.LookPath
	if a.lookupPath != nil {
		lookupPath = a.lookupPath
	}
	resolved, err := lookupPath(command)
	if err != nil {
		return "", fmt.Errorf("%w: pkcheck is not installed or not in PATH; install polkit/polkitd or disable %s to use socket-group fallback", ErrAuthorizationUnavailable, PolkitAuthorizationEnv)
	}
	return resolved, nil
}

func (a PolkitAuthorizer) runCommand(ctx context.Context, command string, args []string) error {
	if a.run != nil {
		return a.run(ctx, command, args)
	}
	cmd := exec.CommandContext(ctx, command, args...)
	return cmd.Run()
}

func authorizerFromEnv() Authorizer {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv(PolkitAuthorizationEnv)))
	switch mode {
	case "", "0", "false", "no", "off", "disabled":
		return AllowAuthorizer{}
	case "1", "true", "yes", "on", "required", "polkit":
		return PolkitAuthorizer{AllowUserInteraction: true}
	default:
		return StaticErrorAuthorizer{Err: fmt.Errorf("%w: invalid %s value %q; use disabled or required", ErrAuthorizationUnavailable, PolkitAuthorizationEnv, mode)}
	}
}

type peerSubjectContextKey struct{}

func contextWithPeerSubject(ctx context.Context, subject PeerSubject) context.Context {
	return context.WithValue(ctx, peerSubjectContextKey{}, subject)
}

func peerSubjectFromContext(ctx context.Context) (PeerSubject, bool) {
	subject, ok := ctx.Value(peerSubjectContextKey{}).(PeerSubject)
	if !ok || subject.PID <= 0 {
		return PeerSubject{}, false
	}
	return subject, true
}

func authorizeHTTPRequest(r *http.Request, authorizer Authorizer, action AuthorizationAction) error {
	if authorizer == nil {
		authorizer = AllowAuthorizer{}
	}
	var subject PeerSubject
	if requiresPeerCredentials(authorizer) {
		var ok bool
		subject, ok = peerSubjectFromContext(r.Context())
		if !ok {
			return fmt.Errorf("%w: daemon could not identify local peer process for %s", ErrAuthorizationUnavailable, action)
		}
	}
	return authorizer.Authorize(r.Context(), action, subject)
}

func requiresPeerCredentials(authorizer Authorizer) bool {
	requirement, ok := authorizer.(peerCredentialRequirement)
	return !ok || requirement.RequiresPeerCredentials()
}

func writeAuthorizationHTTPError(w http.ResponseWriter, err error) {
	status := http.StatusForbidden
	if errors.Is(err, ErrAuthorizationUnavailable) {
		status = http.StatusServiceUnavailable
	}
	http.Error(w, err.Error(), status)
}
