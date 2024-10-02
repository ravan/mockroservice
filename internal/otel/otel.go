package otel

import (
	"context"
	"errors"
	"github.com/ravan/microservice-sim/internal/config"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func InitializeOpenTelemetry(ctx context.Context, cfg config.OtelConfig) (shutdown func(context.Context) error, outErr error) {
	var shutdownFuncs []func(context.Context) error
	// Each registered cleanup will be invoked once.
	shutdown = func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	if !cfg.Trace.Enabled && !cfg.Metrics.Enabled {
		return shutdown, nil
	} else if err := cfg.Validate(); err != nil {
		return shutdown, err
	}

	handleErr := func(inErr error) {
		outErr = errors.Join(inErr, shutdown(ctx))
	}

	otel.SetTextMapPropagator(newPropagator())

	if cfg.Trace.Enabled {
		tp, err := setupTraceProvider(ctx, cfg.Trace)
		if err != nil {
			handleErr(err)
			return
		}
		shutdownFuncs = append(shutdownFuncs, tp.Shutdown)
		otel.SetTracerProvider(tp)
	}

	if cfg.Metrics.Enabled {
		mp, err := setupMetricProvider(ctx, cfg.Metrics)
		if err != nil {
			handleErr(err)
			return
		}
		shutdownFuncs = append(shutdownFuncs, mp.Shutdown)
		otel.SetMeterProvider(mp)
	}

	return shutdown, outErr
}

func setupTraceProvider(ctx context.Context, cfg config.TraceConfig) (*sdktrace.TracerProvider, error) {
	var traceExporter sdktrace.SpanExporter
	if exp, err := setupHttpTraceExporter(ctx, cfg); err != nil {
		return nil, err
	} else if exp != nil {
		traceExporter = exp
	}

	if exp, err := setupGrpcTraceExporter(ctx, cfg); err != nil {
		return nil, err
	} else if exp != nil {
		traceExporter = exp
	}

	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(traceExporter, sdktrace.WithBatchTimeout(5*time.Second)))
	return tp, nil
}

func setupMetricProvider(ctx context.Context, cfg config.MetricsConfig) (*sdkmetric.MeterProvider, error) {
	var metricsExporter sdkmetric.Exporter
	if exp, err := setupHttpMetricsExporter(ctx, cfg); err != nil {
		return nil, err
	} else if exp != nil {
		metricsExporter = exp
	}

	if exp, err := setupGrpcMetricsExporter(ctx, cfg); err != nil {
		return nil, err
	} else if exp != nil {
		metricsExporter = exp
	}

	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricsExporter, sdkmetric.WithInterval(1*time.Minute))))
	return mp, nil
}

func setupHttpTraceExporter(ctx context.Context, cfg config.TraceConfig) (sdktrace.SpanExporter, error) {
	var opts []otlptracehttp.Option
	if cfg.HttpEndpointURL != "" {
		opts = append(opts, otlptracehttp.WithEndpointURL(cfg.HttpEndpointURL))
	} else if cfg.HttpEndpoint != "" {
		opts = append(opts, otlptracehttp.WithEndpoint(cfg.HttpEndpoint))
	}

	if len(opts) == 0 {
		return nil, nil
	}

	if cfg.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	return otlptracehttp.New(ctx, opts...)
}

func setupGrpcTraceExporter(ctx context.Context, cfg config.TraceConfig) (sdktrace.SpanExporter, error) {
	var opts []otlptracegrpc.Option
	if cfg.GrpcEndpointURL != "" {
		opts = append(opts, otlptracegrpc.WithEndpointURL(cfg.GrpcEndpointURL))
	} else if cfg.GrpcEndpoint != "" {
		opts = append(opts, otlptracegrpc.WithEndpoint(cfg.GrpcEndpoint))
	}

	if len(opts) == 0 {
		return nil, nil
	}

	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	return otlptracegrpc.New(ctx, opts...)
}

func setupHttpMetricsExporter(ctx context.Context, cfg config.MetricsConfig) (sdkmetric.Exporter, error) {
	var opts []otlpmetrichttp.Option
	if cfg.HttpEndpointURL != "" {
		opts = append(opts, otlpmetrichttp.WithEndpointURL(cfg.HttpEndpointURL))
	} else if cfg.HttpEndpoint != "" {
		opts = append(opts, otlpmetrichttp.WithEndpoint(cfg.HttpEndpoint))
	}

	if len(opts) == 0 {
		return nil, nil
	}

	if cfg.Insecure {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}

	return otlpmetrichttp.New(ctx, opts...)
}

func setupGrpcMetricsExporter(ctx context.Context, cfg config.MetricsConfig) (sdkmetric.Exporter, error) {
	var opts []otlpmetricgrpc.Option
	if cfg.GrpcEndpointURL != "" {
		opts = append(opts, otlpmetricgrpc.WithEndpointURL(cfg.GrpcEndpointURL))
	} else if cfg.GrpcEndpoint != "" {
		opts = append(opts, otlpmetricgrpc.WithEndpoint(cfg.GrpcEndpoint))
	}

	if len(opts) == 0 {
		return nil, nil
	}

	if cfg.Insecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}

	return otlpmetricgrpc.New(ctx, opts...)
}

func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}
