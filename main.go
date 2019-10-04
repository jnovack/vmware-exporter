package main

import (
	"flag"
	"net/http"
	"os"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/magiconair/properties"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	port := flag.Int("port", 9094, "Port to attach exporter")
	flag.Parse()

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", redirect)

	log.Info("Serving metrics on " + strconv.FormatInt(int64(*port), 10))
	log.Fatal(http.ListenAndServe(":"+strconv.FormatInt(int64(*port), 10), nil))
}

// Redirect
func redirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/metrics", 301)
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

	prometheus.MustRegister(NewvCollector())
}
