package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/AidarKhusainov/tunwarden/internal/client"
)

func TestServerExposesStatusOverUnixSocket(t *testing.T) {
	runtimeDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- (Server{RuntimeDir: runtimeDir}).Run(ctx) }()

	statusClient := client.StatusClient{SocketPath: runtimeDir + "/tunwardend.sock", Timeout: time.Second}
	var daemon string
	for i := 0; i < 50; i++ {
		status, err := statusClient.Status(context.Background())
		if err == nil {
			daemon = status.Daemon
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("server shutdown failed: %v", err)
	}
	if daemon != "running" {
		t.Fatalf("expected daemon running status, got %q", daemon)
	}
}
