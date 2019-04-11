package main

import (
	"flag"
	log "github.com/Sirupsen/logrus"
	"github.com/magiconair/properties"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"strconv"
)

type Configuration struct {
	Host     string
	User     string
	Password string
	Debug    bool
}

var cfg Configuration

func main() {
	port := flag.Int("port", 8080, "Port to attach exporter")
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
	loadConfig()
	prometheus.MustRegister(NewvCollector())
}

// Load Configuration data
func loadConfig() {
	p := properties.MustLoadFiles([]string{
		"config.properties",
	}, properties.UTF8, true)

	cfg = Configuration{Host: p.MustGetString("host"), User: p.MustGetString("user"), Password: p.MustGetString("password"), Debug: p.MustGetBool("debug")}

}
