package config

import (
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
	Address   string     `mapstructure:"address" validate:"required"`
	Port      int        `mapstructure:"port" validate:"required"`
	Endpoints []Endpoint `mapstructure:"endpoints"`
	MemStress MemStress  `mapstructure:"memstress" `
	StressNg  StressNg   `mapstructure:"stressng" `
}

type Endpoint struct {
	Uri         string                 `mapstructure:"uri" validate:"required"`
	Delay       string                 `mapstructure:"delay" `
	ErrorOnCall int                    `mapstructure:"errorOnCall"`
	Body        map[string]interface{} `mapstructure:"body" `
	Routes      []Route                `mapstructure:"routes" `
}

type StressNg struct {
	Enabled bool     `mapstructure:"enabled" `
	Args    []string `mapstructure:"args" `
}
type MemStress struct {
	Enabled    bool   `mapstructure:"enabled"`
	MemSize    string `mapstructure:"memSize" validate:"required_with=Enabled"`
	GrowthTime string `mapstructure:"growthTime" validate:"required_with=Enabled"`
}

type Route struct {
	Uri        string `mapstructure:"uri" validate:"required"`
	Delay      string `mapstructure:"delay" `
	StopOnFail bool   `mapstructure:"stopOnFail"`
}

func GetConfig(configFile string) (*Configuration, error) {
	c := &Configuration{
		MemStress: MemStress{},
		StressNg:  StressNg{},
	}
	v := viper.New()
	v.SetDefault("address", "0.0.0.0")
	v.SetDefault("port", 8080)
	v.SetDefault("stressng.enabled", false)
	v.SetDefault("stressng.args", []string{"-c", "0", "-l", "10"})
	v.SetDefault("memstress.enabled", false)
	v.SetDefault("memstress.memsize", "10%")
	v.SetDefault("memstress.growthtime", "10s")

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
	return c, nil
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

func ParseDelay(delay string) *Delay {

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
