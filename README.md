# gorilla/mux Prometheus easy-to-use instrumentation

## Install
`go get -u github.com/rusart/muxprom`


## Example
```go
package main

import (
	"github.com/gorilla/mux"
	"github.com/rusart/muxprom"
	"net/http"
)

const listen string = ":9000"

var prom *muxprom.MuxProm

func main() {
	router := mux.NewRouter().StrictSlash(true)
	prom = muxprom.New(
		muxprom.Router(router),
	)
	prom.Instrument()

	http.ListenAndServe(listen, router)
}
```

## Options
Setting options example
```go
prom = muxprom.New(
    muxprom.Router(router),
    muxprom.MetricsRouteName("prommetrics"),
    muxprom.MetricsPath("/health/metrics"),
)

```

|Option|Description|
|---|---|
|MetricsPath|Path to the exported metrics. Default: `/metrics`|
|MetricsRouteName|Route name for the exported metrics. Default: `metrics`. Need to override if default already in use|
|DurationBucket|Bucket for request duration metric. Default: `[]float64{.0001, .0005, .001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}`|
|RespSizeBucket|Bucket for response size metric. Default: `[]float64{0, 512, bytefmt.KILOBYTE, 100 * bytefmt.KILOBYTE, 512 * bytefmt.KILOBYTE, bytefmt.MEGABYTE, 5 * bytefmt.MEGABYTE, 10 * bytefmt.MEGABYTE, 25 * bytefmt.MEGABYTE, 50 * bytefmt.MEGABYTE, 100 * bytefmt.MEGABYTE, 500 * bytefmt.MEGABYTE}`|
|Namespace|Prometheus namespace. Default: `muxprom`|

## Grafana Dashboard
https://grafana.com/grafana/dashboards/11976
