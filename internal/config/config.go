package config

import (
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/ravan/microservice-sim/internal/util"
	"github.com/spf13/viper"
	"log/slog"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

type Configuration struct {
	ServiceName   string       `mapstructure:"serviceName" `
	Address       string       `mapstructure:"address" validate:"required"`
	Port          int          `mapstructure:"port" validate:"required"`
	LogLevel      string       `mapstructure:"logLevel"`
	Logging       util.Logging `mapstructure:"logging"`
	Certificate   Certificate  `mapstructure:"certificate"`
	Endpoints     []Endpoint   `mapstructure:"endpoints"`
	MemStress     MemStress    `mapstructure:"memstress" `
	StressNg      StressNg     `mapstructure:"stressng" `
	OpenTelemetry OtelConfig   `mapstructure:"otel"`
}

type Certificate struct {
	Enabled       bool   `mapstructure:"enabled"`
	Delay         string `mapstructure:"delay" `
	CertFile      string `mapstructure:"certificate" validate:"required_with=Enabled"`
	KeyFile       string `mapstructure:"key" validate:"required_with=Enabled"`
	mutex         sync.Mutex
	delayDuration *util.Delay
}

func (c *Certificate) GetDelayDuration() *util.Delay {
	if c.delayDuration == nil {
		c.mutex.Lock()
		c.delayDuration = util.ParseDelay(c.Delay)
		c.mutex.Unlock()
	}
	return c.delayDuration
}

type Endpoint struct {
	Uri           string                 `mapstructure:"uri" validate:"required"`
	Delay         string                 `mapstructure:"delay" `
	ErrorOnCall   int                    `mapstructure:"errorOnCall"`
	ErrorLogging  util.Logging           `mapstructure:"errorLogging"`
	Logging       util.Logging           `mapstructure:"logging"`
	Body          map[string]interface{} `mapstructure:"body" `
	Routes        []Route                `mapstructure:"routes" `
	mutex         sync.Mutex
	delayDuration *util.Delay
}

func (e *Endpoint) GetDelayDuration() *util.Delay {
	if e.delayDuration == nil {
		e.mutex.Lock()
		e.delayDuration = util.ParseDelay(e.Delay)
		e.mutex.Unlock()
	}
	return e.delayDuration
}

type StressNg struct {
	Enabled bool     `mapstructure:"enabled" `
	Delay   string   `mapstructure:"delay"`
	Args    []string `mapstructure:"args" `
}

type MemStress struct {
	Enabled    bool   `mapstructure:"enabled"`
	Delay      string `mapstructure:"delay"`
	MemSize    string `mapstructure:"memSize" validate:"required_with=Enabled"`
	GrowthTime string `mapstructure:"growthTime" validate:"required_with=Enabled"`
}

type Route struct {
	Uri           string       `mapstructure:"uri" validate:"required"`
	Delay         string       `mapstructure:"delay" `
	StopOnFail    bool         `mapstructure:"stopOnFail"`
	Logging       util.Logging `mapstructure:"logging"`
	mutex         sync.Mutex
	delayDuration *util.Delay
}

func (r *Route) GetDelayDuration() *util.Delay {
	if r.delayDuration == nil {
		r.mutex.Lock()
		r.delayDuration = util.ParseDelay(r.Delay)
		r.mutex.Unlock()
	}
	return r.delayDuration
}

type OtelConfig struct {
	Trace   TraceConfig   `mapstructure:"trace" `
	Metrics MetricsConfig `mapstructure:"metrics" `
}

type TraceConfig struct {
	Enabled         bool   `mapstructure:"enabled" `
	TracerName      string `mapstructure:"tracer-name" `
	HttpEndpoint    string `mapstructure:"http-endpoint" `
	HttpEndpointURL string `mapstructure:"http-endpoint-url" `
	GrpcEndpoint    string `mapstructure:"grpc-endpoint" `
	GrpcEndpointURL string `mapstructure:"grpc-endpoint-url" `
	Insecure        bool   `mapstructure:"insecure" `
}

type MetricsConfig struct {
	Enabled         bool   `mapstructure:"enabled" `
	HttpEndpoint    string `mapstructure:"http-endpoint" `
	HttpEndpointURL string `mapstructure:"http-endpoint-url" `
	GrpcEndpoint    string `mapstructure:"grpc-endpoint" `
	GrpcEndpointURL string `mapstructure:"grpc-endpoint-url" `
	Insecure        bool   `mapstructure:"insecure" `
}

func GetConfig(configFile string) (*Configuration, error) {
	c := &Configuration{
		MemStress: MemStress{},
		StressNg:  StressNg{},
	}
	v := viper.New()
	v.SetDefault("serviceName", "SimService")
	v.SetDefault("address", "0.0.0.0")
	v.SetDefault("port", 8080)
	v.SetDefault("logLevel", "info")
	v.SetDefault("logging.logOnCall", 1)
	v.SetDefault("stressng.enabled", false)
	v.SetDefault("stressng.delay", "")
	v.SetDefault("stressng.args", []string{"-c", "0", "-l", "10"})
	v.SetDefault("memstress.enabled", false)
	v.SetDefault("memstress.delay", "")
	v.SetDefault("memstress.memsize", "10%")
	v.SetDefault("memstress.growthtime", "10s")
	v.SetDefault("otel.trace.enabled", false)
	v.SetDefault("otel.metrics.enabled", false)

	v.BindEnv("otel.trace.enabled", "OTEL_EXPORTER_OTLP_TRACES_ENABLED")                //nolint:errcheck
	v.BindEnv("otel.trace.tracer-name", "OTEL_EXPORTER_OTLP_TRACES_TRACER_NAME")        //nolint:errcheck
	v.BindEnv("otel.trace.http-endpoint", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")         //nolint:errcheck
	v.BindEnv("otel.trace.http-endpoint-url", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT_URL") //nolint:errcheck
	v.BindEnv("otel.trace.grpc-endpoint", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")         //nolint:errcheck
	v.BindEnv("otel.trace.grpc-endpoint-url", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT_URL") //nolint:errcheck
	v.BindEnv("otel.trace.insecure", "OTEL_EXPORTER_OTLP_TRACES_INSECURE")              //nolint:errcheck

	v.BindEnv("otel.metrics.enabled", "OTEL_EXPORTER_OTLP_METRICS_ENABLED")                //nolint:errcheck
	v.BindEnv("otel.metrics.http-endpoint", "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT")         //nolint:errcheck
	v.BindEnv("otel.metrics.http-endpoint-url", "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT_URL") //nolint:errcheck
	v.BindEnv("otel.metrics.grpc-endpoint", "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT")         //nolint:errcheck
	v.BindEnv("otel.metrics.grpc-endpoint-url", "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT_URL") //nolint:errcheck
	v.BindEnv("otel.metrics.insecure", "OTEL_EXPORTER_OTLP_METRICS_INSECURE")              //nolint:errcheck

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	if configFile != "" {
		d, f := path.Split(configFile)
		if d == "" {
			d = "."
		}
		v.SetConfigName(f[0 : len(f)-len(filepath.Ext(f))])
		v.SetConfigType("toml")
		v.AddConfigPath(d)
		err := v.ReadInConfig()
		if err != nil {
			slog.Error("Error when reading config file.", slog.Any("error", err))
		}
	}

	if err := v.Unmarshal(c); err != nil {
		slog.Error("Error unmarshalling config", slog.Any("err", err))
		return nil, err
	}

	validate := validator.New(validator.WithRequiredStructEnabled())
	err := validate.Struct(c)
	if err != nil {
		return nil, err
	}
	err = c.OpenTelemetry.Validate()
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c OtelConfig) Validate() error {
	if c.Trace.Enabled && countSet(c.Trace.HttpEndpointURL, c.Trace.GrpcEndpointURL, c.Trace.HttpEndpoint, c.Trace.GrpcEndpoint) != 1 {
		return fmt.Errorf("exactly one http or grpc endpoint is required when opentelemetry tracing is enabled")
	}

	if c.Metrics.Enabled && countSet(c.Metrics.HttpEndpointURL, c.Metrics.GrpcEndpointURL, c.Metrics.HttpEndpoint, c.Metrics.GrpcEndpoint) != 1 {
		return fmt.Errorf("exactly one http or grpc endpoint is required when opentelemetry metrics is enabled")
	}

	return nil
}

func countSet(s ...string) int {
	count := 0
	for _, v := range s {
		if v != "" {
			count++
		}
	}

	return count
}
