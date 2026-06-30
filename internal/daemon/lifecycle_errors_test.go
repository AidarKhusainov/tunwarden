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
	connectErr      error
	disconnectErr   error
	status          api.StatusResponse
	connectCalls    int
	disconnectCalls int
}

func (l *staticLifecycle) Connect(_ context.Context, req api.ConnectRequest) (api.LifecycleResponse, error) {
	l.connectCalls++
	if l.connectErr != nil {
		return api.LifecycleResponse{}, l.connectErr
	}
	return api.LifecycleResponse{Connection: "active", Mode: req.Mode, Proxy: "listening", TUN: "disabled"}, nil
}

func (l *staticLifecycle) Disconnect(context.Context) (api.LifecycleResponse, error) {
	l.disconnectCalls++
	if l.disconnectErr != nil {
		return api.LifecycleResponse{}, l.disconnectErr
	}
	return api.LifecycleResponse{Connection: "inactive", Proxy: "inactive", TUN: "disabled"}, nil
}

func (l *staticLifecycle) Status(context.Context) api.StatusResponse {
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

func TestConnectHandlerBadRequestDoesNotCallLifecycle(t *testing.T) {
	lifecycle := &staticLifecycle{}
	mux := http.NewServeMux()
	registerLifecycleHandlers(mux, lifecycle)

	req := httptest.NewRequest(http.MethodPost, api.ConnectPath, strings.NewReader(`{"mode":"proxy-only"`))
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
	if lifecycle.connectCalls != 0 {
		t.Fatalf("connect lifecycle called %d time(s), want 0", lifecycle.connectCalls)
	}
}

func TestConnectHandlerConflictUsesActiveLifecycleState(t *testing.T) {
	lifecycle := &staticLifecycle{status: api.StatusResponse{Connection: "active"}}
	mux := http.NewServeMux()
	registerLifecycleHandlers(mux, lifecycle)

	req := httptest.NewRequest(http.MethodPost, api.ConnectPath, strings.NewReader(validConnectBody(planner.ModeProxyOnly)))
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusConflict, rr.Body.String())
	}
	if lifecycle.connectCalls != 0 {
		t.Fatalf("connect lifecycle called %d time(s), want 0", lifecycle.connectCalls)
	}
}

func TestConnectHandlerAuthorizationDeniedUsesAccessDeniedCategory(t *testing.T) {
	lifecycle := &staticLifecycle{}
	mux := http.NewServeMux()
	registerLifecycleHandlers(mux, lifecycle, StaticErrorAuthorizer{Err: ErrAuthorizationDenied})

	req := httptest.NewRequest(http.MethodPost, api.ConnectPath, strings.NewReader(validConnectBody(planner.ModeProxyOnly)))
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusForbidden, rr.Body.String())
	}
	if lifecycle.connectCalls != 0 {
		t.Fatalf("connect lifecycle called %d time(s), want 0", lifecycle.connectCalls)
	}
}

func TestConnectHandlerInternalFailureDoesNotUseMessageText(t *testing.T) {
	lifecycle := &staticLifecycle{connectErr: errors.New("connection already active; run podlaz disconnect before connecting another profile")}
	mux := http.NewServeMux()
	registerLifecycleHandlers(mux, lifecycle)

	req := httptest.NewRequest(http.MethodPost, api.ConnectPath, strings.NewReader(validConnectBody(planner.ModeProxyOnly)))
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusInternalServerError, rr.Body.String())
	}
	if lifecycle.connectCalls != 1 {
		t.Fatalf("connect lifecycle called %d time(s), want 1", lifecycle.connectCalls)
	}
}

func validConnectBody(mode string) string {
	return `{"mode":"` + mode + `","profile":{"id":"profile-1","name":"vpn","source":"manual","engine":"xray","server":"vpn.example","port":443,"protocol":"vless"}}`
}
