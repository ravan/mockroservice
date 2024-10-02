package server

import (
	"encoding/json"
	"fmt"
	"github.com/ravan/microservice-sim/internal/config"
	"github.com/ravan/microservice-sim/internal/stress"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

var counters = make(map[string]*Counter)

func Run(conf *config.Configuration) error {
	addr := fmt.Sprintf("%s:%d", conf.Address, conf.Port)
	initEndpoints(conf.Endpoints)
	go initMemStress(&conf.MemStress)
	go initStressNg(&conf.StressNg)
	return http.ListenAndServe(addr, nil)
}

func initEndpoints(endpoints []config.Endpoint) {
	for _, endpoint := range endpoints {
		counters[endpoint.Uri] = &Counter{
			errorAfter:   endpoint.ErrorOnCall,
			errorEnabled: endpoint.ErrorOnCall > 0,
		}
		http.HandleFunc(endpoint.Uri, func(w http.ResponseWriter, r *http.Request) {
			counter := counters[endpoint.Uri]
			if counter.errorEnabled {
				counter.Increment()
				if counter.ShouldError() {
					w.WriteHeader(http.StatusInternalServerError)
					err := json.NewEncoder(w).Encode(map[string]interface{}{
						"message": "An error occurred",
					})
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
					}
					counter.Reset()
					return
				}
			}
			handleEndpoint(&endpoint, w, r)
		})
	}
}

func handleEndpoint(endpoint *config.Endpoint, w http.ResponseWriter, r *http.Request) {
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
			err := handleRoute(&route)
			if err != nil && route.StopOnFail {
				http.Error(w, err.Error(), http.StatusInternalServerError)
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
	} else {
		_, err = w.Write(data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func handleRoute(route *config.Route) error {
	delay := config.ParseDelay(route.Delay)
	if delay.Enabled && delay.BeforeDuration.Milliseconds() > 0 {
		slog.Info("wait", "ms", delay.BeforeDuration.Milliseconds())
		time.Sleep(delay.BeforeDuration)
	}
	slog.Info("calling", "http", route.Uri)
	_, err := http.Get(fmt.Sprintf("http://%s", route.Uri))
	if delay.Enabled && delay.AfterDuration.Milliseconds() > 0 {
		slog.Info("wait", "ms", delay.AfterDuration.Milliseconds())
		time.Sleep(delay.AfterDuration)
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
