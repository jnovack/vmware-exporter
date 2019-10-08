package main

import (
	"flag"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/magiconair/properties"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

// Configuration TODO Comment
type Configuration struct {
	Host     string
	User     string
	Password string
	Debug    bool
	vmStats  bool
}

var cfg Configuration

var defaultTimeout time.Duration

func main() {
	port := flag.Int("port", 9094, "port to bind exporter")
	flag.Parse()

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>VMware Exporter</title></head>
			<body>
			<h1>VMware Exporter</h1>
			<p><a href="/metrics">Metrics</a></p>
			</body>
			</html>`))
	})
	log.Info("Serving metrics on " + strconv.FormatInt(int64(*port), 10))
	log.Fatal(http.ListenAndServe(":"+strconv.FormatInt(int64(*port), 10), nil))
}

func init() {

	defaultTimeout = 30 * time.Second

	// Get config details
	if os.Getenv("HOST") != "" && os.Getenv("USERID") != "" && os.Getenv("PASSWORD") != "" {
		if os.Getenv("DEBUG") == "True" {
			cfg = Configuration{Host: os.Getenv("HOST"), User: os.Getenv("USERID"), Password: os.Getenv("PASSWORD"), Debug: true}
		} else {
			cfg = Configuration{Host: os.Getenv("HOST"), User: os.Getenv("USERID"), Password: os.Getenv("PASSWORD"), Debug: false}
		}
		if os.Getenv("VMSTATS") == "False" {
			cfg = Configuration{Host: os.Getenv("HOST"), User: os.Getenv("USERID"), Password: os.Getenv("PASSWORD"), Debug: true, vmStats: false}
		} else {
			cfg = Configuration{Host: os.Getenv("HOST"), User: os.Getenv("USERID"), Password: os.Getenv("PASSWORD"), Debug: false, vmStats: true}
		}

	} else {
		p := properties.MustLoadFiles([]string{
			"config.properties",
		}, properties.UTF8, true)

		cfg = Configuration{Host: p.MustGetString("host"), User: p.MustGetString("user"), Password: p.MustGetString("password"), Debug: p.MustGetBool("debug"), vmStats: p.MustGetBool("vmstats")}
	}

	prometheus.MustRegister(NewCollector())
}
