package config

import (
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
	"log/slog"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Configuration struct {
	ServiceName   string     `mapstructure:"service-name" `
	Address       string     `mapstructure:"address" validate:"required"`
	Port          int        `mapstructure:"port" validate:"required"`
	Endpoints     []Endpoint `mapstructure:"endpoints"`
	MemStress     MemStress  `mapstructure:"memstress" `
	StressNg      StressNg   `mapstructure:"stressng" `
	OpenTelemetry OtelConfig `mapstructure:"otel"`
}

type Endpoint struct {
	Uri           string                 `mapstructure:"uri" validate:"required"`
	Delay         string                 `mapstructure:"delay" `
	ErrorOnCall   int                    `mapstructure:"errorOnCall"`
	Body          map[string]interface{} `mapstructure:"body" `
	Routes        []Route                `mapstructure:"routes" `
	DelayDuration *Delay
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
	Uri           string `mapstructure:"uri" validate:"required"`
	Delay         string `mapstructure:"delay" `
	StopOnFail    bool   `mapstructure:"stopOnFail"`
	DelayDuration *Delay
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
	v.SetDefault("service-name", "SimService")
	v.SetDefault("address", "0.0.0.0")
	v.SetDefault("port", 8080)
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

	ParseDelays(c)

	return c, nil
}

func ParseDelays(c *Configuration) {
	for i := range c.Endpoints {
		c.Endpoints[i].DelayDuration = parseDelay(c.Endpoints[i].Delay)
		for x := range c.Endpoints[i].Routes {
			c.Endpoints[i].Routes[x].DelayDuration = parseDelay(c.Endpoints[i].Routes[x].Delay)
		}
	}
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

type Delay struct {
	Enabled        bool
	BeforeDuration time.Duration
	AfterDuration  time.Duration
}

const (
	aroundPattern = "^<([0-9a-z]*)>$"
	beforePattern = "^([0-9a-z]*)[<]?"
	afterPattern  = "[>]([0-9a-z]*)$"
	bothPattern   = "([0-9a-z]*)[<][>]([0-9a-z]*)"
)

var (
	aroundRegexp = regexp.MustCompile(aroundPattern)
	beforeRegexp = regexp.MustCompile(beforePattern)
	afterRegexp  = regexp.MustCompile(afterPattern)
	bothRegexp   = regexp.MustCompile(bothPattern)
)

func parseDelay(delay string) *Delay {

	before := "0ms"
	after := "0ms"
	if delay != "" {
		if aroundMatch := aroundRegexp.FindStringSubmatch(delay); aroundMatch != nil {
			before = aroundMatch[1]
			after = aroundMatch[1]
		} else if bothMatch := bothRegexp.FindStringSubmatch(delay); bothMatch != nil {
			before = bothMatch[1]
			after = bothMatch[2]
		} else if beforeMatch := beforeRegexp.FindStringSubmatch(delay); beforeMatch != nil {
			before = beforeMatch[1]
		} else if afterMatch := afterRegexp.FindStringSubmatch(delay); afterMatch != nil {
			after = afterMatch[1]
		}
	}

	disabled := false
	beforeDuration, err := time.ParseDuration(before)
	if err != nil {
		disabled = true
		slog.Error("failed to parse duration", "delay", delay, "duration", before)
	}
	afterDuration, err := time.ParseDuration(after)
	if err != nil {
		disabled = true
		slog.Error("failed to parse duration", "delay", delay, "duration", after)
	}
	return &Delay{
		Enabled:        !disabled,
		BeforeDuration: beforeDuration,
		AfterDuration:  afterDuration,
	}
}
