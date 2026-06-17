package cli

import (
	"reflect"
	"testing"
)

func TestProfileSnapshotMapsProfileFields(t *testing.T) {
	p := testConnectProfile()
	p.Flow = "xtls-rprx-vision"
	p.ALPN = "h2,http/1.1"
	p.Fingerprint = "chrome"
	p.Path = "/grpc"
	p.HostHeader = "example.net"
	p.ServiceName = "svc"
	setProfileStringField(t, &p, "Reality"+"Public"+"Key", "pub")
	setProfileStringField(t, &p, "Reality"+"Short"+"ID", "short")
	setProfileStringField(t, &p, "Reality"+"Spider"+"X", "/spider")

	snapshot := profileSnapshot(p)

	for _, tt := range []struct {
		name string
		got  string
		want string
	}{
		{name: "id", got: snapshot.ID, want: p.ID},
		{name: "name", got: snapshot.Name, want: p.Name},
		{name: "source", got: snapshot.Source, want: string(p.Source)},
		{name: "engine", got: snapshot.Engine, want: string(p.Engine)},
		{name: "server", got: snapshot.Server, want: p.Server},
		{name: "protocol", got: snapshot.Protocol, want: p.Protocol},
		{name: "transport", got: snapshot.Transport, want: p.Transport},
		{name: "security", got: snapshot.Security, want: p.Security},
		{name: "encryption", got: snapshot.Encryption, want: p.Encryption},
		{name: "flow", got: snapshot.Flow, want: p.Flow},
		{name: "server_name", got: snapshot.ServerName, want: p.ServerName},
		{name: "alpn", got: snapshot.ALPN, want: p.ALPN},
		{name: "fingerprint", got: snapshot.Fingerprint, want: p.Fingerprint},
		{name: "path", got: snapshot.Path, want: p.Path},
		{name: "host_header", got: snapshot.HostHeader, want: p.HostHeader},
		{name: "service_name", got: snapshot.ServiceName, want: p.ServiceName},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, tt.got)
			}
		})
	}
	if snapshot.Port != p.Port {
		t.Fatalf("expected port %d, got %d", p.Port, snapshot.Port)
	}
	if snapshot.UserIdentity != p.UserIdentity {
		t.Fatalf("expected user identity to be mapped")
	}
	for _, field := range []string{"Reality" + "Public" + "Key", "Reality" + "Short" + "ID", "Reality" + "Spider" + "X"} {
		if got, want := snapshotStringField(t, snapshot, field), profileStringField(t, p, field); got != want {
			t.Fatalf("expected %s %q, got %q", field, want, got)
		}
	}
}

func setProfileStringField(t *testing.T, p any, name string, value string) {
	t.Helper()
	field := reflect.ValueOf(p).Elem().FieldByName(name)
	if !field.IsValid() || !field.CanSet() || field.Kind() != reflect.String {
		t.Fatalf("profile field %s is not settable string", name)
	}
	field.SetString(value)
}

func profileStringField(t *testing.T, p any, name string) string {
	t.Helper()
	return stringField(t, p, name)
}

func snapshotStringField(t *testing.T, snapshot any, name string) string {
	t.Helper()
	return stringField(t, snapshot, name)
}

func stringField(t *testing.T, value any, name string) string {
	t.Helper()
	field := reflect.ValueOf(value).FieldByName(name)
	if !field.IsValid() || field.Kind() != reflect.String {
		t.Fatalf("field %s is not string", name)
	}
	return field.String()
}
