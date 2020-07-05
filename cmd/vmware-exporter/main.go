package main

import (
	"net/http"
	"os"
	"strconv"
	"time"

	build "github.com/jnovack/go-version"
	vmwareCollector "github.com/jnovack/vmware-exporter/internal/vmware_collector"
	"github.com/mattn/go-isatty"
	"github.com/namsral/flag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var port = flag.Int("port", 9272, "port to bind exporter")
var loglevel = flag.Int("loglevel", 1, "log level (0=debug, 1=info, 2=warn, 3=error")

func main() {

	prometheus.MustRegister(vmwareCollector.NewCollector())

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><title>VMware Exporter</title></head>
			<body><h1>VMware Exporter</h1><h4>` +
			build.Application + ` ` + build.Version +
			`</h4><p><a href="/metrics">Metrics</a></p>
			</body></html>`))
	})

	log.Info().Msgf("Serving metrics on " + strconv.FormatInt(int64(*port), 10))
	http.ListenAndServe(":"+strconv.FormatInt(int64(*port), 10), nil)
}

func init() {
	if isatty.IsTerminal(os.Stdout.Fd()) {
		// Format using ConsoleWriter if running straight
		zerolog.TimestampFunc = func() time.Time {
			return time.Now().In(time.Local)
		}
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	} else {
		// Format using JSON if running as a service (or container)
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	}
	flag.Parse()

	var zerologlevel zerolog.Level
	switch *loglevel {
	case 0:
		zerologlevel = zerolog.DebugLevel
	case 2:
		zerologlevel = zerolog.WarnLevel
	case 3:
		zerologlevel = zerolog.ErrorLevel
	default:
		zerologlevel = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(zerologlevel)
}
