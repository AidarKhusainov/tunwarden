package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/AidarKhusainov/tunwarden/internal/api"
)

type RecoveryClient struct {
	SocketPath string
	Timeout    time.Duration
}

func (c RecoveryClient) Recover(ctx context.Context) (api.RecoveryResponse, error) {
	socketPath := c.SocketPath
	if socketPath == "" {
		socketPath = api.SocketPath("")
	}
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 750 * time.Millisecond
	}

	dialer := net.Dialer{Timeout: timeout}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}
	defer transport.CloseIdleConnections()

	httpClient := http.Client{Transport: transport, Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://tunwardend"+api.RecoverPath, nil)
	if err != nil {
		return api.RecoveryResponse{}, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return api.RecoveryResponse{}, fmt.Errorf("%w: %s", ErrDaemonUnavailable, unavailableDetail(socketPath, err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		message := ""
		if data, readErr := io.ReadAll(resp.Body); readErr == nil {
			message = strings.TrimSpace(string(data))
		}
		return api.RecoveryResponse{}, api.LifecycleHTTPError("recover", resp.Status, message)
	}

	var recovery api.RecoveryResponse
	if err := json.NewDecoder(resp.Body).Decode(&recovery); err != nil {
		return api.RecoveryResponse{}, fmt.Errorf("daemon recover response was invalid: %w", err)
	}
	if err := api.ValidateRecoveryResponse(recovery); err != nil {
		return api.RecoveryResponse{}, fmt.Errorf("daemon recover response was invalid: %w", err)
	}
	return recovery, nil
}
