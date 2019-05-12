package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
	"sync"
	"time"
)

const xver = "1.0"

type vCollector struct {
	desc string
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
	wg.Add(7)

	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("vmware_exporter_version", "go-vmware-export Version", []string{}, prometheus.Labels{"version": xver}),
		prometheus.GaugeValue,
		1,
	)

	go func() {
		defer wg.Done()
		start := time.Now()
		cm := DSMetrics()
		log.Info("DSMetrics: " + time.Since(start).String())
		for _, m := range cm {

			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(m.name, m.help, []string{}, m.labels),
				prometheus.GaugeValue,
				float64(m.value),
			)
		}

	}()

	go func() {
		defer wg.Done()
		start := time.Now()

		cm := ClusterMetrics()
		log.Info("ClusterMetrics: " + time.Since(start).String())
		for _, m := range cm {
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(m.name, m.help, []string{}, m.labels),
				prometheus.GaugeValue,
				float64(m.value),
			)
		}
	}()

	go func() {
		defer wg.Done()
		start := time.Now()

		cm := ClusterCounters()
		log.Info("ClusterCounters: " + time.Since(start).String())
		for _, m := range cm {
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(m.name, m.help, []string{}, m.labels),
				prometheus.CounterValue,
				float64(m.value),
			)
		}
	}()

	go func() {
		defer wg.Done()
		start := time.Now()

		cm := HostMetrics()
		log.Info("HostMetrics: " + time.Since(start).String())
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

	go func() {
		defer wg.Done()
		start := time.Now()

		cm := HostCounters()
		log.Info("HostCounters: " + time.Since(start).String())
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

	go func() {
		defer wg.Done()
		start := time.Now()

		cm := VmMetrics()
		log.Info("VMMetrics: " + time.Since(start).String())
		for _, m := range cm {
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(m.name, m.help, []string{}, m.labels),
				prometheus.GaugeValue,
				float64(m.value),
			)
		}

	}()

	go func() {
		defer wg.Done()
		start := time.Now()

		cm := HostHBAStatus()
		log.Info("HBAStatus: " + time.Since(start).String())
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

func NewvCollector() *vCollector {
	return &vCollector{
		desc: "vmware Exporter",
	}
}

func GetClusterMetricMock() []vMetric {
	ms := []vMetric{
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
