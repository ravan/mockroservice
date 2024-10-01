package config

import (
	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
	"log/slog"
	"path"
	"path/filepath"
	"strings"
)

type Configuration struct {
	Address   string     `mapstructure:"address" validate:"required"`
	Port      int        `mapstructure:"port" validate:"required"`
	Endpoints []Endpoint `mapstructure:"endpoints"`
	MemStress MemStress  `mapstructure:"memstress" `
	StressNg  StressNg   `mapstructure:"stressng" `
}

type Endpoint struct {
	Uri    string                 `mapstructure:"uri" validate:"required"`
	Delay  string                 `mapstructure:"delay" `
	Status int                    `mapstructure:"status" `
	Body   map[string]interface{} `mapstructure:"body" `
	Routes []Route                `mapstructure:"routes" `
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
	Uri        string                 `mapstructure:"uri" validate:"required"`
	Delay      string                 `mapstructure:"delay" `
	Body       map[string]interface{} `mapstructure:"body" `
	StopOnFail bool                   `mapstructure:"stopOnFail"`
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
