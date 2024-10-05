package cmd

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	tempDir := t.TempDir()
	confFile := fmt.Sprintf("%s/conf.toml", tempDir)
	err := os.WriteFile(confFile, []byte(testConfig), 0644)
	require.NoError(t, err)

	err = processMultipartConfig(confFile, "my-test-chart", tempDir)
	require.NoError(t, err)
	chartDir := fmt.Sprintf("%s/my-test-chart", tempDir)
	require.DirExists(t, chartDir)
	err = filepath.WalkDir(tempDir, func(path string, d fs.DirEntry, err error) error {
		fmt.Println(path)
		if strings.HasSuffix(path, "t-rex-tracker-cm.yaml") {
			file, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fmt.Println(string(file))
		}
		return nil
	})
	require.NoError(t, err)
}

const testConfig = `
# Triceratops Transport Service - Latency Culprit
serviceName = "Triceratops Transport"
# Stress simulation: Apply memory stress causing latency issues
[memstress]
enabled = true
delay = "10s"
memSize = "15%" # Memory usage grows
growthTime = "5s"

# Stress the CPU as well to simulate overloading
[stressng]
enabled = true
delay = "20s"
args = ["-c", "0", "-l", "50"] # 50% CPU usage

# Define transport route endpoint with added latency
[[endpoints]]
uri = "/route"
delay = "2s<>5s" # Simulate variable delay before/after processing
errorOnCall = 5 # Errors on every 5th call
body.status = "delayed"
body.msg = "Triceratops Transport is slow today!"

+++

# T-Rex Tracker
serviceName = "T-Rex Tracker"

[[endpoints]]
uri = "/track"
delay = "<100ms>"
body.status = "ok"
body.msg = "T-Rex safely tracked"

[[endpoints.routes]]
uri = "Triceratops Transport/route"
delay = "50ms"  # T-Rex Tracker depends on Triceratops to report tracking data

+++

# Herbivore Hideout
serviceName = "Herbivore Hideout"
[[endpoints]]
uri = "/grazing"
delay = "<200ms>"
body.status = "ok"
body.msg = "Herbivores grazing"

[[endpoints.routes]]
uri = "Triceratops Transport/route"
delay = "100ms"  # Herbivore Hideout also depends on Triceratops to transport herbivores

+++

# PteroPost
serviceName = "PteroPost"

[[endpoints]]
uri = "/send"
delay = "<300ms>"
body.status = "ok"
body.msg = "Message sent to dinosaurs"

[[endpoints.routes]]
uri = "Triceratops Transport/route"
delay = "500ms"  # PteroPost needs Triceratops to transport its messages

[[endpoints.routes]]
uri = "Herbivore Hideout/grazing"
delay = "300ms"  # PteroPost sends status updates to Herbivore Hideout

+++

# Bronto Burger
serviceName = "Bronto Burger"
[[endpoints]]
uri = "/order"
delay = "<150ms>"
body.status = "ok"
body.msg = "Order received"

[[endpoints.routes]]
uri = "PteroPost/send"
delay = "200ms"  # Bronto Burger sends order details to PteroPost for communication with other services

`
