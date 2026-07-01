package metrics

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		c.Next()

		status := c.Writer.Status()
		duration := time.Since(start)

		RequestsTotal.WithLabelValues(
			c.Request.Method,
			path,
			strconv.Itoa(status),
		).Inc()

		RequestDuration.WithLabelValues(
			c.Request.Method,
			path,
		).Observe(duration.Seconds())

		slog.Info("request",
			"method", c.Request.Method,
			"path", path,
			"status", status,
			"duration_ms", duration.Milliseconds(),
		)
	}
}
