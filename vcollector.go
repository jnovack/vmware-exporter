package main

import (
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
)

const xver = "1.0"

type vCollector struct {
	desc string
}

func timeTrack(ch chan<- prometheus.Metric, start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("%s took %.3fs", name, float64(elapsed.Milliseconds())/1000)

	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("go_task_time", "Go task elasped time", []string{}, prometheus.Labels{"task": name, "application": "vmware_exporter"}),
		prometheus.GaugeValue,
		float64(elapsed.Milliseconds())/1000,
	)
}

func (c *vCollector) Describe(ch chan<- *prometheus.Desc) {

	metrics := make(chan prometheus.Metric)
	go func() {
		c.Collect(metrics)
		close(metrics)
	}()
	for m := range metrics {
		ch <- m.Desc()
	}
}

func (c *vCollector) Collect(ch chan<- prometheus.Metric) {

	wg := sync.WaitGroup{}
	if cfg.vmStats == true {
		wg.Add(6)
	} else {
		wg.Add(5)
	}

	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("vmware_exporter", "github.com/jnovack/vmware_exporter", []string{}, prometheus.Labels{"version": xver}),
		prometheus.GaugeValue,
		1,
	)

	// Datastore Metrics
	go func() {
		defer wg.Done()
		defer timeTrack(ch, time.Now(), "DataStoreMetrics")
		cm := DataStoreMetrics()
		for _, m := range cm {

			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(m.name, m.help, []string{}, m.labels),
				prometheus.GaugeValue,
				float64(m.value),
			)
		}

	}()

	// Cluster Metrics
	go func() {
		defer wg.Done()
		defer timeTrack(ch, time.Now(), "ClusterMetrics")
		cm := ClusterMetrics()
		for _, m := range cm {
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(m.name, m.help, []string{}, m.labels),
				prometheus.GaugeValue,
				float64(m.value),
			)
		}
	}()

	/*
		// Cluster Counters
		go func() {
			defer wg.Done()
			defer timeTrack(ch, time.Now(), "ClusterCounters")
			cm := ClusterCounters()
			for _, m := range cm {
				ch <- prometheus.MustNewConstMetric(
					prometheus.NewDesc(m.name, m.help, []string{}, m.labels),
					prometheus.CounterValue,
					float64(m.value),
				)
			}
		}()
	*/

	// Host Metrics
	go func() {
		defer wg.Done()
		defer timeTrack(ch, time.Now(), "HostMetrics")
		cm := HostMetrics()
		for _, m := range cm {
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(m.name, m.help, []string{"cluster", "host"}, nil),
				prometheus.GaugeValue,
				float64(m.value),
				m.labels["cluster"],
				m.labels["host"],
			)
		}
	}()

	// Host Counters
	go func() {
		defer wg.Done()
		defer timeTrack(ch, time.Now(), "HostCounters")
		cm := HostCounters()
		for _, m := range cm {
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(m.name, m.help, []string{"cluster", "host"}, nil),
				prometheus.GaugeValue,
				float64(m.value),
				m.labels["cluster"],
				m.labels["host"],
			)
		}
	}()

	// VM Metrics
	if cfg.vmStats == true {
		go func() {
			defer wg.Done()
			defer timeTrack(ch, time.Now(), "VMMetrics")
			cm := VMMetrics()
			for _, m := range cm {
				ch <- prometheus.MustNewConstMetric(
					prometheus.NewDesc(m.name, m.help, []string{}, m.labels),
					prometheus.GaugeValue,
					float64(m.value),
				)
			}

		}()
	}

	// HBA Status
	go func() {
		defer wg.Done()
		defer timeTrack(ch, time.Now(), "HostHBAStatus")
		cm := HostHBAStatus()
		for _, m := range cm {
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(m.name, m.help, []string{}, m.labels),
				prometheus.GaugeValue,
				float64(m.value),
			)
		}

	}()

	wg.Wait()
}

// NewvCollector TODO Comment
func NewvCollector() *vCollector {
	return &vCollector{
		desc: "vmware Exporter",
	}
}

// GetClusterMetricMock TODO Comment
func GetClusterMetricMock() []VMetric {
	ms := []VMetric{
		{name: "cluster_mem_ballooned", help: "Usage for a", value: 10, labels: map[string]string{"cluster": "apa", "host": "04"}},
		{name: "cluster_mem_compressed", help: "Usage for a", value: 10, labels: map[string]string{"cluster": "apa", "host": "04"}},
		{name: "cluster_mem_consumedOverhead", help: "Usage for a", value: 10, labels: map[string]string{}},
		{name: "cluster_mem_distributedMemoryEntitlement", help: "Usage for a", value: 10, labels: map[string]string{}},
		{name: "cluster_mem_guest", help: "Usage for a", value: 10, labels: map[string]string{}},
		{name: "cluster_mem_usage", help: "Usage for a", value: 10, labels: map[string]string{}},
		{name: "cluster_mem_overhead", help: "Usage for a", value: 10, labels: map[string]string{}},
		{name: "cluster_mem_private", help: "Usage for a", value: 10, labels: map[string]string{}},
		{name: "cluster_mem_staticMemoryEntitlement", help: "Usage for a", value: 10, labels: map[string]string{}},
		{name: "cluster_mem_limit", help: "Usage for a", value: 10, labels: map[string]string{}},
	}
	return ms
}
