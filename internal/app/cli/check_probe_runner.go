package cli

import (
	"context"
	"time"

	profilecheck "github.com/AidarKhusainov/podlaz/internal/check"
)

type checkProbeRunner struct {
	serverTCP func(context.Context, string, time.Duration) profilecheck.ProbeResult
	socks     func(context.Context, profilecheck.Endpoint, profilecheck.Target, time.Duration) profilecheck.ProbeResult
	httpProxy func(context.Context, profilecheck.Endpoint, profilecheck.Target, time.Duration) profilecheck.ProbeResult
}
