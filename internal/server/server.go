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

const (
	traceParent = "traceparent"
	traceState  = "tracestate"
)

func Run(conf *config.Configuration) error {
	ctx := context.Background()
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
	addr := fmt.Sprintf("%s:%d", conf.Address, conf.Port)
	initEndpoints(ctx, conf.ServiceName, conf.Endpoints)
	go initMemStress(&conf.MemStress)
	go initStressNg(&conf.StressNg)
	return http.ListenAndServe(addr, nil)
}

func initEndpoints(ctx context.Context, serviceName string, endpoints []config.Endpoint) {
	for _, endpoint := range endpoints {
		counters[endpoint.Uri] = &Counter{
			errorAfter:   endpoint.ErrorOnCall,
			errorEnabled: endpoint.ErrorOnCall > 0,
		}
		http.HandleFunc(endpoint.Uri, func(w http.ResponseWriter, r *http.Request) {
			ctx, span := otel.Tracer.Start(ctx, fmt.Sprintf("%s.%s", serviceName, endpoint.Uri))
			defer span.End()
			counter := counters[endpoint.Uri]
			if counter.errorEnabled {
				counter.Increment()
				if counter.ShouldError() {
					w.WriteHeader(http.StatusInternalServerError)
					simErr := fmt.Errorf("error while processing: %s", endpoint.Uri)
					span.RecordError(simErr)
					span.SetStatus(codes.Error, simErr.Error())
					err := json.NewEncoder(w).Encode(map[string]interface{}{
						"message": simErr.Error(),
					})
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
					}
					counter.Reset()
					return
				}
			}
			handleEndpoint(ctx, serviceName, &span, &endpoint, w, r)
		})
	}
}

func handleEndpoint(ctx context.Context, serviceName string, span *trace.Span, endpoint *config.Endpoint, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	slog.Info(fmt.Sprintf("%s %s", r.Method, r.URL.Path))
	delay := config.ParseDelay(endpoint.Delay)
	if delay.Enabled && delay.BeforeDuration.Milliseconds() > 0 {
		slog.Info("wait", "ms", delay.BeforeDuration.Milliseconds())
		time.Sleep(delay.BeforeDuration)
	}
	if len(endpoint.Routes) > 0 {
		for _, route := range endpoint.Routes {
			err := handleRoute(ctx, serviceName, &route)
			if err != nil && route.StopOnFail {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				(*span).RecordError(err)
				(*span).SetStatus(codes.Error, err.Error())
				return
			}
		}
	}
	if delay.Enabled && delay.AfterDuration.Milliseconds() > 0 {
		slog.Info("wait", "ms", delay.AfterDuration.Milliseconds())
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		(*span).RecordError(err)
		(*span).SetStatus(codes.Error, err.Error())
	} else {
		_, err = w.Write(data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			(*span).RecordError(err)
			(*span).SetStatus(codes.Error, err.Error())
		}
	}
}

func handleRoute(ctx context.Context, serviceName string, route *config.Route) error {
	ctx, span := otel.Tracer.Start(ctx, fmt.Sprintf("%s.Route", serviceName))
	defer span.End()

	tc := propagation.TraceContext{}
	mc := propagation.MapCarrier{}

	tc.Inject(ctx, mc)
	client := &http.Client{}

	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s", route.Uri), nil)
	if err != nil {
		return err
	}

	if _, ok := mc[traceParent]; ok && mc[traceParent] != "" {
		req.Header.Add(traceParent, mc.Get(traceParent))
	}
	if _, ok := mc[traceState]; ok {
		req.Header.Add(traceState, mc.Get(traceState))
	}

	delay := config.ParseDelay(route.Delay)
	if delay.Enabled && delay.BeforeDuration.Milliseconds() > 0 {
		slog.Info("wait", "ms", delay.BeforeDuration.Milliseconds())
		time.Sleep(delay.BeforeDuration)
	}
	slog.Info("calling", "http", route.Uri)
	_, err = client.Do(req)

	if delay.Enabled && delay.AfterDuration.Milliseconds() > 0 {
		slog.Info("wait", "ms", delay.AfterDuration.Milliseconds())
		time.Sleep(delay.AfterDuration)
	}
	span.RecordError(err)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

func initMemStress(conf *config.MemStress) {
	if conf.Enabled {
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
		slog.Info("stressing", "args", strings.Join(conf.Args, ", "))
		stress.Stress(conf.Args)
	}
}
