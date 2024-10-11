package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ravan/microservice-sim/internal/config"
	"github.com/ravan/microservice-sim/internal/otel"
	"github.com/ravan/microservice-sim/internal/stress"
	"github.com/ravan/microservice-sim/internal/util"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

var errorCounters = make(map[string]*util.Counter)
var client = &http.Client{}
var otelActive = false
var serviceName = "service-sim"
var envVars map[string]string

const (
	traceParent = "traceparent"
	traceState  = "tracestate"
)

func Run(conf *config.Configuration) error {
	setDefaultLogLevel(conf.LogLevel)
	envVars = getEnvironmentVars()
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

	mux := http.NewServeMux()

	initEndpoints(mux, conf.Endpoints)
	go initMemStress(&conf.MemStress)
	go initStressNg(&conf.StressNg)
	data := getDataMap()
	conf.Logging.LogBefore(data)
	conf.Logging.LogAfter(data)

	var handler http.Handler
	if otelActive {
		httpSpanName := func(operation string, r *http.Request) string {
			uri := strings.Replace(r.URL.Path, "/", ".", -1)
			if string(uri[0]) == "." {
				uri = uri[1:]
			}
			return fmt.Sprintf("%s.%s", serviceName, uri)
		}
		handler = otelhttp.NewHandler(
			mux,
			"/",
			otelhttp.WithSpanNameFormatter(httpSpanName),
		)
	}

	slog.Info("Listening on", slog.String("address", addr))
	return http.ListenAndServe(addr, handler)
}

func getDataMap() map[string]interface{} {
	return map[string]interface{}{
		"Env":         envVars,
		"ServiceName": serviceName,
	}
}
func initEndpoints(mux *http.ServeMux, endpoints []config.Endpoint) {
	for i := range endpoints {
		endpoint := &endpoints[i]
		errorCounters[endpoint.Uri] = &util.Counter{
			TriggerOn: endpoint.ErrorOnCall,
			Active:    endpoint.ErrorOnCall > 0,
		}
		mux.HandleFunc(endpoint.Uri, func(w http.ResponseWriter, r *http.Request) {
			endpointHandler(endpoint, w, r)
		})
	}
}

func endpointHandler(endpoint *config.Endpoint, w http.ResponseWriter, r *http.Request) {
	data := getDataMap()
	data["Endpoint"] = endpoint
	ctx := r.Context()

	if handleErrorSimulation(endpoint, w, data) {
		return
	}
	endpoint.Logging.LogBefore(data)
	handleEndpoint(&ctx, endpoint, w, r)
	endpoint.Logging.LogAfter(data)
}

func handleErrorSimulation(endpoint *config.Endpoint, w http.ResponseWriter, data map[string]interface{}) bool {
	counter := errorCounters[endpoint.Uri]
	if counter.Active {
		counter.Increment()
		if counter.ShouldTrigger() {
			w.WriteHeader(http.StatusInternalServerError)
			errMsg := endpoint.ErrorLogging.GetLogBeforeMsg(data)
			if errMsg == "" {
				errMsg = fmt.Sprintf("error while processing: %s", endpoint.Uri)
			}

			simErr := fmt.Errorf(errMsg)
			writeErrorResponseBody(w, simErr)
			counter.Reset()
			slog.Error(errMsg)
			slog.Debug("err simulation", "triggered-nth-call", counter.TriggerOn)
			return true
		}
	}
	return false
}

func writeErrorResponseBody(w http.ResponseWriter, simErr error) {
	err := json.NewEncoder(w).Encode(map[string]interface{}{
		"message": simErr.Error(),
	})
	if err != nil {
		setupInternalServerError(w, err)
	}
}

func handleEndpoint(ctx *context.Context, endpoint *config.Endpoint, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	data := getDataMap()

	slog.Debug(fmt.Sprintf("%s %s", r.Method, r.URL.Path))
	endpoint.GetDelayDuration().ApplyBefore("routing", "self")
	if len(endpoint.Routes) > 0 {
		for i := range endpoint.Routes {
			route := &endpoint.Routes[i]
			data["Route"] = route
			route.Logging.LogBefore(data)
			err := handleRoute(ctx, route)
			if err != nil && route.StopOnFail {
				setupInternalServerError(w, err)
				slog.Error("Error when calling.", "target", route.Uri, slog.String("error", err.Error()))
				return
			}
			route.Logging.LogAfter(data)
		}
	} else {
		slog.Debug("no routes defined")
	}

	endpoint.GetDelayDuration().ApplyAfter("routing", "self")
	writeSuccessResponseBody(endpoint, w)
}

func writeSuccessResponseBody(endpoint *config.Endpoint, w http.ResponseWriter) {
	body := map[string]interface{}{
		"success": true,
	}
	if len(endpoint.Body) > 0 {
		for k, v := range endpoint.Body {
			body[k] = v
		}
	}
	b, err := json.Marshal(body)
	if err != nil {
		setupInternalServerError(w, err)
	} else {
		_, err = w.Write(b)
		if err != nil {
			setupInternalServerError(w, err)
		}
	}
}

func setupInternalServerError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
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
	route.GetDelayDuration().ApplyBefore("route-call", route.Uri)
	slog.Debug("calling", "target", route.Uri)
	_, err = client.Do(req)
	slog.Debug("returned", "target", route.Uri, slog.Any("error", err))
	route.GetDelayDuration().ApplyAfter("route-call", route.Uri)

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
				slog.Debug("mem stress start delay.", "delay", startDelay)
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
				slog.Debug("stress start delay.", "delay", startDelay)
				time.Sleep(startDelay)
			}
		}
		slog.Info("stressing", "args", strings.Join(conf.Args, ", "))
		stress.Stress(conf.Args)
	}
}

func setDefaultLogLevel(stringLevel string) {
	level := slog.LevelInfo
	switch strings.ToLower(stringLevel) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	handler := slog.NewTextHandler(log.Writer(), &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))

}

func getEnvironmentVars() map[string]string {
	items := make(map[string]string)
	for _, item := range os.Environ() {
		splits := strings.Split(item, "=")
		items[splits[0]] = splits[1]
	}
	return items
}
