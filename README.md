# MockroService 

Simulate a microservice for observability demos.
Stitch together application topologies controlling each MockroService to mimic a desired behaviour.


## Features

- Memory stress 
- Stress and load the computer system using [stress-ng](https://manpages.ubuntu.com/manpages/focal/man1/stress-ng.1.html)
- Define rest endpoints with ability to route them to other MockroServices
- Define latency and error rate 
- OpenTelemetry support

## Sample Config

```toml

# Listen for http calls at 0.0.0.0:8080 
port= 8080
address= "0.0.0.0"

# When enabled the service with use 10% of available memory. It will take 10 seconds to reach this limit.
[memstress]
enabled = false
delay = "30s"   # wait before starting stress
memSize = "10%" # or size in bytes
growthTime = "10s"

# When enabled the service will apply a stressng command as documented at https://wiki.ubuntu.com/Kernel/Reference/stress-ng
[stressng]
enabled = false
delay = "1m"   # wait before starting stress
args = ["-c", "0", "-l", "10"] # stress cpu at 10%

# Define a "save" endpoint that will delay 1ms before starting processing and wait 1ms after processing.
[[endpoints]]
uri = "/save"
delay = "<1ms>" # format:  before ("1ms", "1ms<"), after (">2s"), both ("2s<>20s", "<5s>")
errorOnCall = 10 # error on every 10th call
body.status = "ok"
body.msg = "saved"

# Define a "list" endpoint that calls the "list" endpoint at host called "product"
[[endpoints]]
uri = "/list"

[[endpoints.routes]]
uri = "another-mockroservice-host/list"  # format: "host:port/endpoint"
delay = "1ms"  # delay before calling
stopOnFail = false

# OpenTelemetry collection information can be configured here or use standard OTEL environment variables
[otel.trace]
enabled = false
tracer-name = "simservice"
# http-endpoint
# http-endpoint-url
# grpc-endpoint
# grpc-endpoint-url
# insecure

[otel.metrics]
enabled = false
# http-endpoint
# http-endpoint-url
# grpc-endpoint
# grpc-endpoint-url
# insecure

```


