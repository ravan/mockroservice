# MockroService 

Simulate a microservice for observability demos.
Stitch together application topologies controlling each MockroService to mimic a desired behaviour.


## Features

- Memory stress 
- Stress and load the computer system using [stress-ng](https://manpages.ubuntu.com/manpages/focal/man1/stress-ng.1.html)
- Define rest endpoints with ability to route them to other MockroServices
- Define latency and error rate 
- Define log messages to simulate functional processing during endpoint and route execution 
  - Messages define using golang templates and [Sprig](https://masterminds.github.io/sprig/) 
- Ability to failed on expired certificate.
- OpenTelemetry support

## Sample Config

```toml

# Listen for http calls at 0.0.0.0:8080 
port= 8080
address= "0.0.0.0"
serviceName= "My Service"
logLevel = "info"   # debug, warn, error

# When enabled will fail if supplied certificate is expired.
[certificate]
enabled = false
delay = "1ms"
certificate = "certs/certificate.pem"
key = "certs/key.pem"

# When enabled the service with use 10% of available memory. It will take 10 seconds to reach this limit.
[memstress]
enabled = false
delay = "30s"   # wait before starting stress
memSize = "10 MB" # humanize size like KB, MB, GB, or a percentage  
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

# Custom log messages can be defined for endpoints. Messages are golang text template using "[[" and "]]" delimiters
# You can access .Env, ServiceName and .Endpoint variables.
[endpoints.logging]
before = "before [[.Endpoint.Uri]]"
beforeLevel = "Warn"  # optional, defaults to info
after = "after [[.Endpoint.Uri]]" #optional
afterLevel =   "Info" # optinal
logOnCall =   1 # Log on every nth call. Use 0 to disable.

# Custom error messages can be defined for endpoints. 
[endpoints.errorLogging]
before = "internal error occurred while processing [[.Endpoint.Uri]]"

# Define a "list" endpoint that calls the "list" endpoint at host called "product"
[[endpoints]]
uri = "/list"

[[endpoints.routes]]
uri = "another-mockroservice-host/list"  # format: "host:port/endpoint"
delay = "1ms"  # delay before calling
stopOnFail = false

# Custom error messages can be defined for routes.
# You can access .Env, ServiceName and .Route variables.
[endpoints.routes.logging]
before = "listing interest rates from [[.Route.Uri]]"
logOnCall = 10   # only log on every 10th call

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
# same connectivity variables as in otel.trace

```

## Generating Helm Chart

MockroService has the ability to generate a helm chart based on multiple service definitions.
You can specify multiple service definitions in a single toml file by separating them with `+++`. Similar to yaml's `---`.

[Sample Services](./sample.toml.mockroservices)

```bash
docker run --rm --name mockroservice -v .:/workspace ravan/mockroservice:0.0.11 /app/sim -c /workspace/sample.toml.mockroservices generate -o /workspace/sample-charts --name my-charts
```

Now you can deploy the services using Helm.

```bash
cd sample-charts/my-chart
helm upgrade --install --create-namespace --namspace museum mockroservice-demo .
```

