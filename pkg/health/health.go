// Package health expone los endpoints de salud comunes a todos los servicios:
//
//   - GET /health/live: el proceso está vivo (siempre 200 mientras responda).
//   - GET /health/ready: el servicio puede atender; verifica cada dependencia
//     propia (Postgres, Mongo…). Si alguna falla responde 503 con el detalle.
//
// El cuerpo de ambos es un Report JSON (no RFC 7807): es un informe de estado
// que el api-gateway agrega en GET /api/v1/status.
package health

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Estados posibles.
const (
	StatusReady    = "ready"
	StatusNotReady = "not_ready"
	StatusUp       = "up"
	StatusDown     = "down"
)

// Check verifica una dependencia. Fn debe respetar la cancelación del context.
type Check struct {
	Name string
	Fn   func(ctx context.Context) error
}

// CheckResult es el resultado de un Check.
type CheckResult struct {
	Status    string `json:"status"`
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// Report es el cuerpo de /health/live y /health/ready.
type Report struct {
	Status string                 `json:"status"`
	Checks map[string]CheckResult `json:"checks"`
}

const maxErrorLen = 200

// Evaluate ejecuta todos los checks en paralelo, cada uno con el timeout dado.
func Evaluate(ctx context.Context, timeout time.Duration, checks ...Check) Report {
	report := Report{Status: StatusReady, Checks: make(map[string]CheckResult, len(checks))}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, chk := range checks {
		wg.Add(1)
		go func(chk Check) {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			start := time.Now()
			err := chk.Fn(cctx)
			res := CheckResult{Status: StatusUp, LatencyMS: time.Since(start).Milliseconds()}
			if err != nil {
				res.Status = StatusDown
				res.Error = describe(cctx, err)
			}

			mu.Lock()
			report.Checks[chk.Name] = res
			if err != nil {
				report.Status = StatusNotReady
			}
			mu.Unlock()
		}(chk)
	}
	wg.Wait()
	return report
}

func describe(ctx context.Context, err error) string {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "timeout"
	}
	msg := err.Error()
	if len(msg) > maxErrorLen {
		msg = msg[:maxErrorLen] + "…"
	}
	return msg
}

// Register agrega /health/live y /health/ready al router.
func Register(r gin.IRoutes, timeout time.Duration, checks ...Check) {
	r.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, Report{Status: "alive", Checks: map[string]CheckResult{}})
	})
	r.GET("/health/ready", func(c *gin.Context) {
		report := Evaluate(c.Request.Context(), timeout, checks...)
		status := http.StatusOK
		if report.Status != StatusReady {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, report)
	})
}
