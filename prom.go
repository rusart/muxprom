package muxprom

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"code.cloudfoundry.org/bytefmt"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var defaultMetricsPath = "/metrics"
var defaultMetricsRouteName = "metrics"
var defaultNamespace = "muxprom"
var defaultDurationBucket = []float64{.0001, .0005, .001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
var defaultRespSizeBucket = []float64{0, 512, bytefmt.KILOBYTE, 100 * bytefmt.KILOBYTE, 512 * bytefmt.KILOBYTE, bytefmt.MEGABYTE, 5 * bytefmt.MEGABYTE, 10 * bytefmt.MEGABYTE, 25 * bytefmt.MEGABYTE, 50 * bytefmt.MEGABYTE, 100 * bytefmt.MEGABYTE, 500 * bytefmt.MEGABYTE}

type statusWriter struct {
	http.ResponseWriter
	status int
	length int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	n, err := w.ResponseWriter.Write(b)
	w.length += n
	return n, err
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	writer, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("not supported by the underlying writer")
	}
	return writer.Hijack()
}

type MuxProm struct {
	reqInFlight          prometheus.GaugeVec
	reqDurationHistogram prometheus.HistogramVec
	reqRespSizeHistogram prometheus.HistogramVec

	Router           *mux.Router
	Namespace        string
	MetricsPath      string
	MetricsRouteName string

	DurationBucket []float64
	RespSizeBucket []float64
}

func Namespace(ns string) func(*MuxProm) {
	return func(prom *MuxProm) {
		prom.Namespace = ns
	}
}

func MetricsPath(p string) func(*MuxProm) {
	return func(prom *MuxProm) {
		prom.MetricsPath = p
	}
}

func MetricsRouteName(rn string) func(*MuxProm) {
	return func(prom *MuxProm) {
		prom.MetricsRouteName = rn
	}
}

func DurationBucket(db []float64) func(*MuxProm) {
	return func(prom *MuxProm) {
		prom.DurationBucket = db
	}
}

func RespSizeBucket(rsb []float64) func(*MuxProm) {
	return func(prom *MuxProm) {
		prom.RespSizeBucket = rsb
	}
}

func Router(r *mux.Router) func(*MuxProm) {
	return func(prom *MuxProm) {
		prom.Router = r
	}
}

func New(options ...func(prom *MuxProm)) *MuxProm {
	p := &MuxProm{
		Namespace:        defaultNamespace,
		MetricsPath:      defaultMetricsPath,
		MetricsRouteName: defaultMetricsRouteName,
		DurationBucket:   defaultDurationBucket,
		RespSizeBucket:   defaultRespSizeBucket,
	}
	for _, option := range options {
		option(p)
	}
	p.init()

	if p.Router != nil {
		p.Router.
			Name(p.MetricsRouteName).
			Methods("GET").
			Path(p.MetricsPath).
			Handler(promhttp.Handler())
	} else {
		log.Fatal("You need to set Router")
	}

	return p
}

func (prom *MuxProm) Instrument() {
	prom.Router.Use(prom.middleware)
	prom.Router.NotFoundHandler = WrapNotFoundHandler(prom.Router.NotFoundHandler, prom.middleware)
	prom.Router.MethodNotAllowedHandler = WrapMethodNotAllowedHandler(prom.Router.MethodNotAllowedHandler, prom.middleware)
}

func (prom *MuxProm) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		routeName := r.URL.RequestURI()
		prom.reqInFlight.WithLabelValues(routeName, r.Method).Inc()
		start := time.Now()
		sw := statusWriter{ResponseWriter: w}
		next.ServeHTTP(&sw, r)
		duration := time.Since(start)
		prom.reqDurationHistogram.WithLabelValues(routeName, r.Method, fmt.Sprintf("%d", sw.status)).Observe(duration.Seconds())
		prom.reqRespSizeHistogram.WithLabelValues(routeName, r.Method, fmt.Sprintf("%d", sw.status)).Observe(float64(sw.length))
		prom.reqInFlight.WithLabelValues(routeName, r.Method).Dec()
	})
}

func (prom *MuxProm) init() {
	prom.reqInFlight = *prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: prom.Namespace,
			Name:      "http_requests_inflight",
			Help:      "HTTP requests in-flight",
		},
		[]string{"route", "method"},
	)
	prometheus.MustRegister(prom.reqInFlight)

	prom.reqDurationHistogram = *prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: prom.Namespace,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration seconds",
			Buckets:   prom.DurationBucket,
		},
		[]string{"route", "method", "http_status"},
	)
	prometheus.MustRegister(prom.reqDurationHistogram)

	prom.reqRespSizeHistogram = *prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: prom.Namespace,
			Name:      "http_response_size",
			Help:      "HTTP response size in bytes",
			Buckets:   prom.RespSizeBucket,
		},
		[]string{"route", "method", "http_status"},
	)
	prometheus.MustRegister(prom.reqRespSizeHistogram)
}

func WrapNotFoundHandler(h http.Handler, m mux.MiddlewareFunc) http.Handler {
	if h == nil {
		h = http.NotFoundHandler()
	}
	return m(h)
}

func WrapMethodNotAllowedHandler(h http.Handler, m mux.MiddlewareFunc) http.Handler {
	if h == nil {
		h = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusMethodNotAllowed)
		})
	}
	return m(h)
}
