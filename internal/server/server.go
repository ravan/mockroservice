package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ravan/microservice-sim/internal/config"
	"github.com/ravan/microservice-sim/internal/otel"
	"github.com/ravan/microservice-sim/internal/stress"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

var counters = make(map[string]*Counter)
var client = &http.Client{}
var otelActive = false
var serviceName = "service-sim"

const (
	traceParent = "traceparent"
	traceState  = "tracestate"
)

func Run(conf *config.Configuration) error {
	otelActive = conf.OpenTelemetry.Trace.Enabled || conf.OpenTelemetry.Metrics.Enabled
	serviceName = conf.ServiceName
	ctx := context.Background()
	if otelActive {
		shutdown, err := otel.InitializeOpenTelemetry(ctx, conf.OpenTelemetry)
		if err != nil {
			return err
		}
		defer func(ctx context.Context) {
			err := shutdown(ctx)
			if err != nil {
				slog.Error("Error shutting down otel:", slog.Any("error", err))
			}
		}(ctx)
		otel.NewTracer(conf.OpenTelemetry)
	}
	addr := fmt.Sprintf("%s:%d", conf.Address, conf.Port)
	initEndpoints(conf.Endpoints)
	go initMemStress(&conf.MemStress)
	go initStressNg(&conf.StressNg)
	slog.Info("Listening on", slog.String("address", addr))
	return http.ListenAndServe(addr, nil)
}

func initEndpoints(endpoints []config.Endpoint) {
	for i := range endpoints {
		endpoint := &endpoints[i]
		counters[endpoint.Uri] = &Counter{
			errorAfter:   endpoint.ErrorOnCall,
			errorEnabled: endpoint.ErrorOnCall > 0,
		}
		http.HandleFunc(endpoint.Uri, func(w http.ResponseWriter, r *http.Request) {
			endpointHandler(endpoint, w, r)
		})
	}
}

func endpointHandler(endpoint *config.Endpoint, w http.ResponseWriter, r *http.Request) {
	var span trace.Span
	ctx := r.Context()
	if otelActive {
		propagator := propagation.TraceContext{}
		ctx = propagator.Extract(ctx, propagation.HeaderCarrier(r.Header))
		ctx, span = otel.Tracer.Start(ctx, getTraceName(serviceName, endpoint))
		defer span.End()
	}
	counter := counters[endpoint.Uri]
	if counter.errorEnabled {
		counter.Increment()
		if counter.ShouldError() {
			w.WriteHeader(http.StatusInternalServerError)
			simErr := fmt.Errorf("error while processing: %s", endpoint.Uri)
			recordSpanError(&span, simErr)
			err := json.NewEncoder(w).Encode(map[string]interface{}{
				"message": simErr.Error(),
			})
			if err != nil {
				setupInternalServerError(w, err, &span)
			}
			counter.Reset()
			slog.Error("Simulated Error", "triggered-nth-call", counter.errorAfter)
			return
		}
	}
	handleEndpoint(&ctx, &span, endpoint, w, r)
}
func getTraceName(serviceName string, endpoint *config.Endpoint) string {
	uri := strings.Replace(endpoint.Uri, "/", ".", -1)
	if string(uri[0]) == "." {
		uri = uri[1:]
	}
	return fmt.Sprintf("%s.%s", serviceName, uri)
}

func handleEndpoint(ctx *context.Context, span *trace.Span, endpoint *config.Endpoint, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		recordSpanError(span, fmt.Errorf("%s is not allowed", r.Method))
		return
	}
	slog.Info(fmt.Sprintf("%s %s", r.Method, r.URL.Path))
	delay := endpoint.DelayDuration
	if delay.Enabled && delay.BeforeDuration.Milliseconds() > 0 {
		slog.Info("latency before routing", "ms", delay.BeforeDuration.Milliseconds())
		time.Sleep(delay.BeforeDuration)
	}
	if len(endpoint.Routes) > 0 {
		for i := range endpoint.Routes {
			route := &endpoint.Routes[i]
			err := handleRoute(ctx, route)
			if err != nil && route.StopOnFail {
				setupInternalServerError(w, err, span)
				slog.Error("Error when calling.", "target", route.Uri, slog.String("error", err.Error()))
				return
			}
		}
	}
	if delay.Enabled && delay.AfterDuration.Milliseconds() > 0 {
		slog.Info("latency after routing", "ms", delay.AfterDuration.Milliseconds())
		time.Sleep(delay.AfterDuration)
	}

	body := map[string]interface{}{
		"success": true,
	}
	if len(endpoint.Body) > 0 {
		for k, v := range endpoint.Body {
			body[k] = v
		}
	}
	data, err := json.Marshal(body)
	if err != nil {
		setupInternalServerError(w, err, span)
	} else {
		_, err = w.Write(data)
		if err != nil {
			setupInternalServerError(w, err, span)
		}
	}
}

func setupInternalServerError(w http.ResponseWriter, err error, span *trace.Span) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
	recordSpanError(span, err)
}

func recordSpanError(span *trace.Span, err error) {
	if otelActive {
		(*span).RecordError(err)
		(*span).SetStatus(codes.Error, err.Error())
	}
}

func handleRoute(ctx *context.Context, route *config.Route) error {
	req, err := http.NewRequestWithContext(*ctx, "GET", fmt.Sprintf("http://%s", route.Uri), nil)
	if err != nil {
		return err
	}
	var span trace.Span
	if otelActive {
		var newCtx context.Context
		newCtx, span = otel.Tracer.Start(*ctx,
			fmt.Sprintf("%s.%s", serviceName, strings.ReplaceAll(route.Uri, "/", ".")))
		defer span.End()

		tc := propagation.TraceContext{}
		mc := propagation.MapCarrier{}

		tc.Inject(newCtx, mc)

		if _, ok := mc[traceParent]; ok && mc[traceParent] != "" {
			req.Header.Add(traceParent, mc.Get(traceParent))
		}
		if _, ok := mc[traceState]; ok {
			req.Header.Add(traceState, mc.Get(traceState))
		}
	}
	delay := route.DelayDuration
	if delay.Enabled && delay.BeforeDuration.Milliseconds() > 0 {
		slog.Info("latency before call", "ms", delay.BeforeDuration.Milliseconds(), "target", route.Uri)
		time.Sleep(delay.BeforeDuration)
	}
	slog.Info("calling", "target", route.Uri)
	_, err = client.Do(req)
	slog.Info("returned", "target", route.Uri, slog.Any("error", err))

	if delay.Enabled && delay.AfterDuration.Milliseconds() > 0 {
		slog.Info("latency after call", "ms", delay.AfterDuration.Milliseconds(), "target", route.Uri)
		time.Sleep(delay.AfterDuration)
	}
	if otelActive && err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

func initMemStress(conf *config.MemStress) {
	if conf.Enabled {
		if conf.Delay != "" {
			startDelay, err := time.ParseDuration(conf.Delay)
			if err != nil {
				slog.Error("Error parsing mem stress start delay", "delay", conf.Delay, "error", err)
			} else {
				slog.Info("mem stress start delay.", "delay", startDelay)
				time.Sleep(startDelay)
			}
		}
		slog.Info("stressing memory", "size", conf.MemSize, "timing", conf.GrowthTime)
		duration, err := time.ParseDuration(conf.GrowthTime)
		if err != nil {
			slog.Error("failed to parse duration", slog.Any("error", err))
			os.Exit(1)
		} else {
			err = stress.Mem(conf.MemSize, duration)
			if err != nil {
				slog.Error("failed to stress memory", slog.Any("error", err))
				os.Exit(1)
			}
		}
	}
}

func initStressNg(conf *config.StressNg) {
	if conf.Enabled {
		if conf.Delay != "" {
			startDelay, err := time.ParseDuration(conf.Delay)
			if err != nil {
				slog.Error("Error parsing stress start delay", "delay", conf.Delay, "error", err)
			} else {
				slog.Info("stress start delay.", "delay", startDelay)
				time.Sleep(startDelay)
			}
		}
		slog.Info("stressing", "args", strings.Join(conf.Args, ", "))
		stress.Stress(conf.Args)
	}
}
