package daemon

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AidarKhusainov/podlaz/internal/api"
	"github.com/AidarKhusainov/podlaz/internal/network/planner"
	"github.com/AidarKhusainov/podlaz/internal/profile"
)

type staticLifecycle struct {
	connectErr         error
	status             api.StatusResponse
	statusAfterConnect api.StatusResponse
	connectCalls       int
}

func (l *staticLifecycle) Connect(_ context.Context, req api.ConnectRequest) (api.LifecycleResponse, error) {
	l.connectCalls++
	if l.connectErr != nil {
		return api.LifecycleResponse{}, l.connectErr
	}
	return api.LifecycleResponse{Connection: "active", Mode: req.Mode, Proxy: "listening", TUN: "disabled"}, nil
}

func (l *staticLifecycle) Disconnect(context.Context) (api.LifecycleResponse, error) {
	return api.LifecycleResponse{Connection: "inactive", Proxy: "inactive", TUN: "disabled"}, nil
}

func (l *staticLifecycle) Status(context.Context) api.StatusResponse {
	if l.connectCalls > 0 && l.statusAfterConnect.Connection != "" {
		return l.statusAfterConnect
	}
	return l.status
}

func TestDaemonAPIHTTPStatusCodeUsesStableCategories(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "bad request", err: daemonAPIBadRequest(errors.New("plain bad request")), want: http.StatusBadRequest},
		{name: "profile validation", err: profile.ValidationError{Messages: []string{"source is required"}}, want: http.StatusBadRequest},
		{name: "conflict", err: daemonAPIConflict(errors.New("plain conflict")), want: http.StatusConflict},
		{name: "active conflict", err: activeConnectionError(), want: http.StatusConflict},
		{name: "full-tunnel active race", err: errFullTunnelConnectionBecameActive, want: http.StatusConflict},
		{name: "access denial", err: daemonAPIAccessDenied(errors.New("plain access denial")), want: http.StatusForbidden},
		{name: "service unavailable", err: daemonAPIServiceUnavailable(errors.New("plain unavailable")), want: http.StatusServiceUnavailable},
		{name: "internal", err: daemonAPIInternal(errors.New("plain internal")), want: http.StatusInternalServerError},
		{name: "uncategorized old conflict text is internal", err: errors.New("connection already active; run podlaz disconnect before connecting another profile"), want: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := daemonAPIHTTPStatusCode(tt.err); got != tt.want {
				t.Fatalf("daemonAPIHTTPStatusCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestConnectHandlerConflictRaceFromLifecycleConnectRemainsConflict(t *testing.T) {
	lifecycle := &staticLifecycle{connectErr: activeConnectionError(), status: api.StatusResponse{Connection: "inactive"}, statusAfterConnect: api.StatusResponse{Connection: "active"}}
	mux := http.NewServeMux()
	registerLifecycleHandlers(mux, lifecycle)

	req := httptest.NewRequest(http.MethodPost, api.ConnectPath, strings.NewReader(validConnectBody(planner.ModeProxyOnly)))
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusConflict, rr.Body.String())
	}
	if lifecycle.connectCalls != 1 {
		t.Fatalf("connect lifecycle called %d time(s), want 1", lifecycle.connectCalls)
	}
}

func TestConnectHandlerSentinelConflictDoesNotRequireActiveStatus(t *testing.T) {
	lifecycle := &staticLifecycle{connectErr: errConnectionAlreadyActive, status: api.StatusResponse{Connection: "inactive"}}
	mux := http.NewServeMux()
	registerLifecycleHandlers(mux, lifecycle)

	req := httptest.NewRequest(http.MethodPost, api.ConnectPath, strings.NewReader(validConnectBody(planner.ModeTun)))
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusConflict, rr.Body.String())
	}
	if lifecycle.connectCalls != 1 {
		t.Fatalf("connect lifecycle called %d time(s), want 1", lifecycle.connectCalls)
	}
}

func validConnectBody(mode string) string {
	return `{"mode":"` + mode + `","profile":{"id":"profile-1","name":"vpn","source":"manual","engine":"xray","server":"vpn.example","port":443,"protocol":"vless"}}`
}
