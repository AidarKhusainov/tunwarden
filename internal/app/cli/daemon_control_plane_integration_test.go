package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AidarKhusainov/tunwarden/internal/api"
	daemonapi "github.com/AidarKhusainov/tunwarden/internal/daemon"
	"github.com/AidarKhusainov/tunwarden/internal/profile"
)

func TestCLIConnectStatusDisconnectWithDaemonAndFakeXray(t *testing.T) {
	dir := t.TempDir()
	runtimeDir := filepath.Join(dir, "runtime")
	stateHome := filepath.Join(dir, "state")
	profileStorePath := filepath.Join(stateHome, "tunwarden", "profiles.json")
	fakeArgsPath := filepath.Join(dir, "fake-xray.args")
	fakeXray := writeLongRunningFakeBinary(t, filepath.Join(dir, "fake-xray"), fakeArgsPath)

	t.Setenv(api.RuntimeDirEnv, runtimeDir)
	t.Setenv(api.XrayPathEnv, fakeXray)
	t.Setenv("XDG_STATE_HOME", stateHome)

	p := cliDaemonIntegrationProfile()
	store, err := profile.NewStore(profileStorePath)
	if err != nil {
		t.Fatalf("create profile store: %v", err)
	}
	if err := store.Add(p); err != nil {
		t.Fatalf("seed profile: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	daemonErr := make(chan error, 1)
	go func() {
		daemonErr <- (daemonapi.Server{RuntimeDir: runtimeDir, Lifecycle: &daemonapi.XrayManager{RuntimeDir: runtimeDir, XrayPath: fakeXray, StopTimeout: 200 * time.Millisecond}}).Run(ctx)
	}()
	waitForDaemonSocket(t, runtimeDir)
	defer func() {
		cancel()
		select {
		case err := <-daemonErr:
			if err != nil {
				t.Fatalf("daemon exited with error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("daemon did not stop")
		}
	}()

	opts := options{profileStorePath: profileStorePath}
	var connectOut bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"connect", "--mode", "proxy-only", p.ID}, &connectOut, opts); err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	connectText := connectOut.String()
	for _, want := range []string{"TunWarden connection started", "Mode: proxy-only", "Connection: active", "TUN: disabled", "Runtime config:"} {
		if !strings.Contains(connectText, want) {
			t.Fatalf("expected connect output to contain %q, got %q", want, connectText)
		}
	}
	for _, secret := range []string{p.UserIdentity, p.RealityPublicKey} {
		if strings.Contains(connectText, secret) {
			t.Fatalf("connect output leaked secret %q in %q", secret, connectText)
		}
	}

	args, err := os.ReadFile(fakeArgsPath)
	if err != nil {
		t.Fatalf("read fake Xray args: %v", err)
	}
	if got := strings.TrimSpace(string(args)); !strings.Contains(got, "run") || !strings.Contains(got, "-config") {
		t.Fatalf("fake Xray did not receive expected run arguments: %q", got)
	}

	configPath := filepath.Join(runtimeDir, "generated", "xray.json")
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected generated Xray config: %v", err)
	}
	var generated map[string]any
	if err := json.Unmarshal(config, &generated); err != nil {
		t.Fatalf("generated Xray config is not valid JSON: %v", err)
	}
	configText := string(config)
	for _, want := range []string{p.Server, p.UserIdentity, p.Transport, p.Security, p.RealityPublicKey, p.RealityShortID} {
		if !strings.Contains(configText, want) {
			t.Fatalf("generated Xray config does not contain expected normalized field %q: %s", want, configText)
		}
	}

	var statusOut bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"status"}, &statusOut, opts); err != nil {
		t.Fatalf("status failed: %v", err)
	}
	statusText := statusOut.String()
	for _, want := range []string{"Daemon: running", "Connection: active", "Mode: proxy-only", "TUN: disabled"} {
		if !strings.Contains(statusText, want) {
			t.Fatalf("expected status output to contain %q, got %q", want, statusText)
		}
	}
	for _, secret := range []string{p.UserIdentity, p.RealityPublicKey, string(config)} {
		if strings.Contains(statusText, secret) {
			t.Fatalf("status output leaked secret/config content %q in %q", secret, statusText)
		}
	}

	var disconnectOut bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"disconnect"}, &disconnectOut, opts); err != nil {
		t.Fatalf("disconnect failed: %v", err)
	}
	if got := disconnectOut.String(); !strings.Contains(got, "TunWarden disconnected") || !strings.Contains(got, "Connection: inactive") {
		t.Fatalf("unexpected disconnect output: %q", got)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("expected generated config to be removed after disconnect, stat err=%v", err)
	}

	var recoverOut bytes.Buffer
	if err := runWithOptions(context.Background(), []string{"recover", "--execute", "--yes"}, &recoverOut, opts); err != nil {
		t.Fatalf("recover after clean disconnect failed: %v", err)
	}
	if got := recoverOut.String(); !strings.Contains(got, "Recovery completed") && !strings.Contains(got, "No TunWarden-owned stale state") {
		t.Fatalf("unexpected recover output after clean disconnect: %q", got)
	}
}

func cliDaemonIntegrationProfile() profile.Profile {
	return profile.Profile{
		ID:               "daemon-vless",
		Name:             "Daemon VLESS",
		Source:           profile.SourceImportedFile,
		Engine:           profile.EngineXray,
		Server:           "daemon-vless.example",
		Port:             443,
		Protocol:         "vless",
		UserIdentity:     "00000000-0000-0000-0000-000000000601",
		Transport:        "tcp",
		Security:         "reality",
		Encryption:       "none",
		Flow:             "xtls-rprx-vision",
		ServerName:       "daemon-vless.example",
		Fingerprint:      "chrome",
		RealityPublicKey: "public-key-daemon",
		RealityShortID:   "abcd",
		RealitySpiderX:   "/",
	}
}

func waitForDaemonSocket(t *testing.T, runtimeDir string) {
	t.Helper()
	socketPath := api.SocketPath(runtimeDir)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(socketPath); err == nil && info.Mode()&os.ModeSocket != 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("daemon socket was not created at %s", socketPath)
}

func writeLongRunningFakeBinary(t *testing.T, path, argsPath string) string {
	t.Helper()
	script := `#!/bin/sh
printf '%s\n' "$@" > "$TUNWARDEN_FAKE_PROCESS_ARGS"
trap 'exit 0' TERM INT
while :; do sleep 1; done
`
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	t.Setenv("TUNWARDEN_FAKE_PROCESS_ARGS", argsPath)
	return path
}
