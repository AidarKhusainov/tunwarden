package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/AidarKhusainov/tunwarden/internal/network/planner"
)

const (
	defaultTunProbeHost = "1.1.1.1"
	defaultTunProbePort = uint16(53)
	defaultProbeTimeout = 3 * time.Second
)

func verifyTunConnectivity(ctx context.Context, plan planner.TunPlan, core tunCoreRuntimePlan) error {
	if plan.TunDevice.Name == "" {
		return errors.New("connectivity probe requires a planned TUN device")
	}
	if core.SOCKSEndpoint == "" {
		return errors.New("connectivity probe requires a private Xray SOCKS endpoint")
	}
	probeCtx, cancel := context.WithTimeout(ctx, defaultProbeTimeout)
	defer cancel()
	if err := socks5TCPConnectProbe(probeCtx, core.SOCKSEndpoint, defaultTunProbeHost, defaultTunProbePort); err != nil {
		return fmt.Errorf("basic TUN-mode connectivity probe through Xray SOCKS endpoint failed: %w", err)
	}
	return nil
}

func socks5TCPConnectProbe(ctx context.Context, socksEndpoint, targetHost string, targetPort uint16) error {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", socksEndpoint)
	if err != nil {
		return fmt.Errorf("dial private Xray SOCKS endpoint %s: %w", socksEndpoint, err)
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		return fmt.Errorf("write SOCKS5 greeting: %w", err)
	}
	greeting := make([]byte, 2)
	if _, err := io.ReadFull(conn, greeting); err != nil {
		return fmt.Errorf("read SOCKS5 greeting response: %w", err)
	}
	if greeting[0] != 0x05 || greeting[1] != 0x00 {
		return fmt.Errorf("SOCKS5 endpoint rejected no-auth greeting: %v", greeting)
	}
	ip := net.ParseIP(targetHost).To4()
	if ip == nil {
		return fmt.Errorf("probe target %s is not an IPv4 address", targetHost)
	}
	request := []byte{0x05, 0x01, 0x00, 0x01, ip[0], ip[1], ip[2], ip[3], byte(targetPort >> 8), byte(targetPort)}
	if _, err := conn.Write(request); err != nil {
		return fmt.Errorf("write SOCKS5 connect request to %s:%s: %w", targetHost, strconv.Itoa(int(targetPort)), err)
	}
	response := make([]byte, 10)
	if _, err := io.ReadFull(conn, response); err != nil {
		return fmt.Errorf("read SOCKS5 connect response: %w", err)
	}
	if response[0] != 0x05 || response[1] != 0x00 {
		return fmt.Errorf("SOCKS5 connect to %s:%d failed with status 0x%02x", targetHost, targetPort, response[1])
	}
	return nil
}
