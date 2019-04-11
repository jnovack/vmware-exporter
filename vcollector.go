package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

type vMetric struct {
	name   string
	help   string
	value  float64
	labels map[string]string
}

type vCollector struct {
	desc string
}

func (c *vCollector) Describe(ch chan<- *prometheus.Desc) {

	cm := DSMetrics()
	for _, m := range cm {
		ch <- prometheus.NewDesc(m.name, m.help, []string{}, m.labels)
	}

	cm = ClusterMetrics()
	for _, m := range cm {
		ch <- prometheus.NewDesc(m.name, m.help, []string{}, m.labels)
	}

	cm = HostMetrics()
	for _, m := range cm {
		ch <- prometheus.NewDesc(m.name, m.help, []string{}, m.labels)
	}
}

func (c *vCollector) Collect(ch chan<- prometheus.Metric) {

	cm := DSMetrics()
	for _, m := range cm {
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(m.name, m.help, []string{}, m.labels),
			prometheus.GaugeValue,
			float64(m.value),
		)
	}

	cm = ClusterMetrics()
	for _, m := range cm {
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(m.name, m.help, []string{}, m.labels),
			prometheus.GaugeValue,
			float64(m.value),
		)
	}

	cm = HostMetrics()
	for _, m := range cm {
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(m.name, m.help, []string{}, m.labels),
			prometheus.GaugeValue,
			float64(m.value),
		)
	}

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
