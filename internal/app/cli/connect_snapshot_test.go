package cli

import "testing"

func TestProfileSnapshotMapsProfileFields(t *testing.T) {
	p := testConnectProfile()
	p.Flow = "xtls-rprx-vision"
	p.ALPN = "h2,http/1.1"
	p.Fingerprint = "chrome"
	p.Path = "/grpc"
	p.HostHeader = "example.net"
	p.ServiceName = "svc"
	p.RealityPublicKey = "pub"
	p.RealityShortID = "short"
	p.RealitySpiderX = "/spider"

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
		{name: "reality_public_key", got: snapshot.RealityPublicKey, want: p.RealityPublicKey},
		{name: "reality_short_id", got: snapshot.RealityShortID, want: p.RealityShortID},
		{name: "reality_spider_x", got: snapshot.RealitySpiderX, want: p.RealitySpiderX},
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
}
