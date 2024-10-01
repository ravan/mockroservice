package server

import (
	"fmt"
	"github.com/ravan/microservice-sim/internal/config"
	"github.com/ravan/microservice-sim/internal/stress"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

func Run(conf *config.Configuration) error {
	addr := fmt.Sprintf("%s:%d", conf.Address, conf.Port)
	initRoutes(conf.Endpoints)
	go initMemStress(&conf.MemStress)
	go initStressNg(&conf.StressNg)
	return http.ListenAndServe(addr, nil)
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
func initRoutes(endpoints []config.Endpoint) {
	for _, endpoint := range endpoints {
		http.HandleFunc(endpoint.Uri, func(w http.ResponseWriter, r *http.Request) {
			handleEndpoint(&endpoint, w, r)
		})
	}
}

func handleEndpoint(endpoint *config.Endpoint, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if len(endpoint.Routes) > 0 {
		for _, route := range endpoint.Routes {
			err := handleRoute(&route)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
	}

}

func handleRoute(route *config.Route) error {
	_, err := http.Get(fmt.Sprintf("http://%s", route.Uri))
	return err
}
