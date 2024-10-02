# Mockroservice 

Application to simulate a microservice to be used in observability workshop scenarios.
Stitch together microservice topologies controlling each mockroservice to mimic a desired behaviour.


## Features

- Ability to memory stress app
- Ability to stress and load the computer system using [stress-ng](https://manpages.ubuntu.com/manpages/focal/man1/stress-ng.1.html)
- Ability to define Rest Endpoints and route them to call other rest services in other mockroservices
- Ability to define latency and error rate 

## Sample Config

```toml

# Listen for http calls at 0.0.0.0:8080 
port= 8080
address= "0.0.0.0"

# When enabled the service with use 10% of available memory. It will take 10 seconds to reach this limit.
[memstress]
enabled = false
memSize = "10%" # or size in bytes
growthTime = "10s"

# When enabled the service will apply a stressng command as documented at https://wiki.ubuntu.com/Kernel/Reference/stress-ng
[stressng]
enabled = false
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
uri = "product/list"  # format: "host:port/endpoint"
delay = "1ms"  # delay before calling
stopOnFail = false


```


