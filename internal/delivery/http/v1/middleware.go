package v1

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLogger — middleware для структурированного логирования HTTP запросов.
func RequestLogger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		log.Info("http request",
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.String("query", c.Request.URL.RawQuery),
			slog.Int("status", c.Writer.Status()),
			slog.String("ip", c.ClientIP()),
			slog.Duration("latency", time.Since(start)),
		)
	}
}
