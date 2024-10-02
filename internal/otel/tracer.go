package otel

import (
	"github.com/ravan/microservice-sim/internal/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

var Tracer trace.Tracer

func NewTracer(cfg config.OtelConfig) {
	if !cfg.Trace.Enabled {
		Tracer = otel.Tracer("")
		return
	}

	Tracer = otel.Tracer(cfg.Trace.TracerName)
}
