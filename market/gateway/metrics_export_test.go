package gateway

import (
	"strings"
	"testing"
)

func TestFormatPrometheusMetricsIncludesCoreCounters(t *testing.T) {
	output := FormatPrometheusMetrics(GatewayMetrics{
		RequestsTotal:       42,
		RequestsActive:      3,
		RequestsFailed:      2,
		RequestsRateLimited: 1,
		AverageLatencyMs:    17,
		PeakLatencyMs:       99,
	})

	for _, want := range []string{
		"gateway_requests_total 42",
		"gateway_requests_active 3",
		"gateway_requests_failed 2",
		"gateway_requests_rate_limited 1",
		"gateway_average_latency_ms 17",
		"gateway_peak_latency_ms 99",
		"# TYPE gateway_requests_total counter",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected prometheus output to contain %q, got:\n%s", want, output)
		}
	}
}
