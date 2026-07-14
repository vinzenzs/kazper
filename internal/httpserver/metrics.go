package httpserver

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// metrics holds the opt-in Prometheus instrumentation. It uses a private
// registry (not the global default) so multiple servers in one process — as in
// tests — never collide on duplicate collector registration. Standard Go
// runtime + process collectors are registered alongside the request metrics.
type metrics struct {
	registry    *prometheus.Registry
	reqTotal    *prometheus.CounterVec
	reqDuration *prometheus.HistogramVec
}

func newMetrics() *metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	labels := []string{"route", "method", "status"}
	m := &metrics{
		registry: reg,
		reqTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests by route template, method, and status class.",
		}, labels),
		reqDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency by route template, method, and status class.",
			Buckets: prometheus.DefBuckets,
		}, labels),
	}
	reg.MustRegister(m.reqTotal, m.reqDuration)
	return m
}

// middleware records request count + duration. The route label is Gin's route
// TEMPLATE (e.g. /api/v1/meals/:id), never the raw id-bearing path, to keep
// label cardinality bounded; status is collapsed to its class (2xx/4xx/5xx).
func (m *metrics) middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched" // NoRoute / SPA fallthrough — never the raw path
		}
		labels := prometheus.Labels{
			"route":  route,
			"method": c.Request.Method,
			"status": strconv.Itoa(c.Writer.Status()/100) + "xx",
		}
		m.reqTotal.With(labels).Inc()
		m.reqDuration.With(labels).Observe(time.Since(start).Seconds())
	}
}

// handler serves the Prometheus text exposition for this registry.
func (m *metrics) handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
