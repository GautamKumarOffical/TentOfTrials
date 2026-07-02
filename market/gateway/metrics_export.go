package gateway

import (
	"strconv"
	"strings"
)

// FormatPrometheusMetrics renders gateway counters as Prometheus text exposition.
func FormatPrometheusMetrics(metrics GatewayMetrics) string {
	lines := []string{
		"# HELP gateway_requests_total Total HTTP requests handled by the gateway.",
		"# TYPE gateway_requests_total counter",
		"gateway_requests_total " + strconv.FormatInt(metrics.RequestsTotal, 10),
		"# HELP gateway_requests_active Currently active HTTP requests.",
		"# TYPE gateway_requests_active gauge",
		"gateway_requests_active " + strconv.FormatInt(metrics.RequestsActive, 10),
		"# HELP gateway_requests_failed Failed HTTP requests.",
		"# TYPE gateway_requests_failed counter",
		"gateway_requests_failed " + strconv.FormatInt(metrics.RequestsFailed, 10),
		"# HELP gateway_requests_rate_limited Rate-limited HTTP requests.",
		"# TYPE gateway_requests_rate_limited counter",
		"gateway_requests_rate_limited " + strconv.FormatInt(metrics.RequestsRateLimited, 10),
		"# HELP gateway_average_latency_ms Rolling average request latency in milliseconds.",
		"# TYPE gateway_average_latency_ms gauge",
		"gateway_average_latency_ms " + strconv.FormatInt(metrics.AverageLatencyMs, 10),
		"# HELP gateway_peak_latency_ms Peak request latency in milliseconds.",
		"# TYPE gateway_peak_latency_ms gauge",
		"gateway_peak_latency_ms " + strconv.FormatInt(metrics.PeakLatencyMs, 10),
	}
	return strings.Join(lines, "\n") + "\n"
}
