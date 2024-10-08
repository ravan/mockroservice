package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ravan/microservice-sim/internal/config"
	"github.com/ravan/microservice-sim/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"text/template"
	"time"
)

func mockServerHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	fmt.Println("Mock Endpoint called: ", path)
}

func getEndpoints(serverUrl string) []config.Endpoint {
	pongUri := strings.Split(serverUrl, "//")[1]
	var endpoints []config.Endpoint
	endpoints = append(endpoints, config.Endpoint{
		Uri: "/ping",
		Body: map[string]interface{}{
			"ping": "pong",
		},
		Delay: "<5ms>",
		Logging: util.Logging{
			Before:      "[[ $options := list \"a \" \"b\" \"c\" ]]\n     [[ $i := randInt 0 3 ]]\n     [[ index $options $i ]]",
			After:       "after [[.Endpoint.Uri]]",
			BeforeLevel: "Warn",
			AfterLevel:  "Info",
			LogOnCall:   1,
		},
		Routes: []config.Route{
			{
				Uri: fmt.Sprintf("%s/pong", pongUri),
				Logging: util.Logging{
					Before:      "before [[.Route.Uri]]",
					After:       "after [[.Route.Uri]]",
					BeforeLevel: "Warning",
					AfterLevel:  "Info",
					LogOnCall:   2,
				},
			},
		},
	})
	return endpoints
}

func TestTemplate(t *testing.T) {
	tpl, err := template.New("test").Parse("")
	require.NoError(t, err)

	var b bytes.Buffer
	writer := bufio.NewWriter(&b)
	err = tpl.Execute(writer, nil)
	require.NoError(t, err)
	fmt.Println(b.String())
}

func TestRun(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(mockServerHandler))
	conf, err := config.GetConfig("")
	require.NoError(t, err)
	conf.LogLevel = "info"
	conf.Endpoints = getEndpoints(server.URL)
	go func() {
		err := Run(conf)
		require.NoError(t, err)
	}()
	time.Sleep(1 * time.Second)
	_, _ = http.Get("http://localhost:8080/ping")
	resp, err := http.Get("http://localhost:8080/ping")
	defer resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	data := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&data)
	require.NoError(t, err)
	assert.Equal(t, "pong", data["ping"])
}

// This test will trigger the process that consumes the memory
// Manually open the activity monitor to see the process appear and consume the memory
func TestMemStress(t *testing.T) {
	conf, err := config.GetConfig("")
	require.NoError(t, err)
	conf.MemStress.Enabled = true
	conf.MemStress.GrowthTime = "1s"
	conf.MemStress.MemSize = "90%"
	go func() {
		err := Run(conf)
		require.NoError(t, err)
	}()
	time.Sleep(5 * time.Second)
}

// This test will trigger the process that consumes the cpu
// Manually open the activity monitor to see the process appear and consume the cpu
func TestStressNg(t *testing.T) {
	conf, err := config.GetConfig("")
	require.NoError(t, err)
	conf.StressNg.Enabled = true
	go func() {
		err := Run(conf)
		require.NoError(t, err)
	}()
	time.Sleep(20 * time.Second)
}

// Launch Jager
// docker run --rm --name jaeger -e COLLECTOR_ZIPKIN_HOST_PORT=:9411 -p 16686:16686 -p 4318:4318 jaegertracing/all-in-one:latest
// http://localhost:16686

func TestOtel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(mockServerHandler))
	conf, err := config.GetConfig("")
	require.NoError(t, err)
	conf.Endpoints = getEndpoints(server.URL)
	conf.OpenTelemetry.Trace = config.TraceConfig{
		Enabled:      true,
		TracerName:   "testTracer",
		HttpEndpoint: "localhost:4318",
		Insecure:     true,
	}
	go func() {
		err := Run(conf)
		require.NoError(t, err)
	}()
	resp, err := http.Get("http://localhost:8080/ping")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	time.Sleep(5 * time.Second)

}
