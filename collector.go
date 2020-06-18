package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// TODO Change to prometheus/common/version
const xver = "1.0"

// Collector TODO Comment
type Collector struct {
	desc string
}

func timeTrack(ch chan<- prometheus.Metric, start time.Time, name string) {
	elapsed := time.Since(start)
	log.Debugf("%s took %.3fs", name, float64(elapsed.Milliseconds())/1000)

	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("go_task_time", "Go task elasped time", []string{}, prometheus.Labels{"task": name, "application": "vmware_exporter"}),
		prometheus.GaugeValue,
		float64(elapsed.Milliseconds())/1000,
	)
}

// Describe sends the super-set of all possible descriptors of metrics collected by this Collector.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {

	metrics := make(chan prometheus.Metric)
	go func() {
		c.Collect(metrics)
		close(metrics)
	}()
	for m := range metrics {
		ch <- m.Desc()
	}
}

// Collect is called by the Prometheus registry when collecting metrics.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {

	wg := sync.WaitGroup{}

	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("vmware_exporter", "github.com/jnovack/vmware_exporter", []string{}, prometheus.Labels{"version": xver}),
		prometheus.GaugeValue,
		1,
	)

	// Datacenter Metrics
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer timeTrack(ch, time.Now(), "DatacenterMetrics")
		cm := DatacenterMetrics(ch)
		for _, m := range cm {

			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(m.name, m.help, []string{}, m.labels),
				prometheus.GaugeValue,
				float64(m.value),
			)
		}

	}()

	// VM Metrics
	if cfg.vmStats == true {
		wg.Add(1)
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
	wg.Add(1)
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

// NewCollector TODO Comment
func NewCollector() *Collector {
	return &Collector{
		desc: "vmware Exporter",
	}
}

// VMetric TODO Comment
type VMetric struct {
	name   string
	help   string
	value  float64
	labels map[string]string
}

// NewClient Connect to vCenter
func NewClient(ctx context.Context) (*govmomi.Client, error) {

	u, err := url.Parse("https://" + cfg.Host + vim25.Path)
	if err != nil {
		log.Fatal(err)
	}
	u.User = url.UserPassword(cfg.User, cfg.Password)
	log.Debug("Connecting to " + u.String())

	return govmomi.NewClient(ctx, u, true)
}

// DatacenterMetrics TODO Comment
func DatacenterMetrics(ch chan<- prometheus.Metric) []VMetric {
	log.SetReportCaller(true)
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	c, err := NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	defer c.Logout(ctx)

	m := view.NewManager(c.Client)

	view, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"Datacenter"}, true)
	if err != nil {
		log.Error(err.Error())
	}

	defer view.Destroy(ctx)

	var metrics []VMetric
	var arrDC []mo.Datacenter

	err = view.Retrieve(ctx, []string{"Datacenter"}, []string{"name", "datastore", "network"}, &arrDC)
	if err != nil {
		log.Error(err.Error())
	}

	waitGroupDC := sync.WaitGroup{}

	for _, objDC := range arrDC {

		waitGroupDC.Add(1)
		go func(objDC mo.Datacenter) {
			defer waitGroupDC.Done()
			defer timeTrack(ch, time.Now(), fmt.Sprintf("DatastoreMetrics - %s", objDC.Name))

			stats := DatastoreMetrics(objDC)
			for _, s := range stats {
				metrics = append(metrics, s)
			}
		}(objDC)

		waitGroupDC.Add(1)
		go func(ch chan<- prometheus.Metric, objDC mo.Datacenter) {
			defer waitGroupDC.Done()
			defer timeTrack(ch, time.Now(), fmt.Sprintf("ClusterMetrics - %s", objDC.Name))

			stats := ClusterMetrics(ch, objDC)
			for _, s := range stats {
				metrics = append(metrics, s)
			}
		}(ch, objDC)

	}

	waitGroupDC.Wait()

	return metrics
}

// DatastoreMetrics TODO Comment
func DatastoreMetrics(objDC mo.Datacenter) []VMetric {
	log.SetReportCaller(true)
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	c, err := NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	defer c.Logout(ctx)

	m := view.NewManager(c.Client)

	vmgr, err := m.CreateContainerView(ctx, objDC.Reference(), []string{"ClusterComputeResource"}, true)
	if err != nil {
		log.Error(err.Error())

	}

	defer vmgr.Destroy(ctx)

	var metrics []VMetric

	var lst []mo.ClusterComputeResource
	err = vmgr.Retrieve(ctx, []string{"ClusterComputeResource"}, []string{"name", "datastore"}, &lst)
	if err != nil {
		log.Error(err.Error())
	}
	for _, cls := range lst {

		cname := cls.Name
		cname = strings.ToLower(cname)

		var dsl []mo.Datastore
		pc := c.PropertyCollector()
		pc.Retrieve(ctx, cls.Datastore, []string{"summary", "name"}, &dsl)

		for _, ds := range dsl {
			if ds.Summary.Accessible {
				dsCapacity := ds.Summary.Capacity
				dsFreeSpace := ds.Summary.FreeSpace
				dsUsed := dsCapacity - dsFreeSpace

				metrics = append(metrics, VMetric{name: "vsphere_datastore_size", help: "Maximum capacity of this datastore, in bytes.", value: float64(dsCapacity), labels: map[string]string{"datastore": ds.Summary.Name, "cluster": cname, "datacenter": objDC.Name}})
				metrics = append(metrics, VMetric{name: "vsphere_datastore_free", help: "Available space of this datastore, in bytes.", value: float64(dsFreeSpace), labels: map[string]string{"datastore": ds.Summary.Name, "cluster": cname, "datacenter": objDC.Name}})
				metrics = append(metrics, VMetric{name: "vsphere_datastore_used", help: "Used space of this datastore, in bytes.", value: float64(dsUsed), labels: map[string]string{"datastore": ds.Summary.Name, "cluster": cname, "datacenter": objDC.Name}})
			}

		}
	}

	return metrics
}

// ClusterMetrics TODO Comment
func ClusterMetrics(ch chan<- prometheus.Metric, objDC mo.Datacenter) []VMetric {
	log.SetReportCaller(true)
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	c, err := NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	defer c.Logout(ctx)

	var arrCLS []mo.ClusterComputeResource
	e2 := GetClusters(ctx, c, &arrCLS)
	if e2 != nil {
		log.Error(e2.Error())
	}

	m := view.NewManager(c.Client)

	view, err := m.CreateContainerView(ctx, objDC.Reference(), []string{"ResourcePool"}, true)
	if err != nil {
		log.Error(err.Error())
	}

	defer view.Destroy(ctx)

	var pools []mo.ResourcePool
	err = view.RetrieveWithFilter(ctx, []string{"ResourcePool"}, []string{"summary", "name", "parent", "config"}, &pools, property.Filter{"name": "Resources"})
	if err != nil {
		log.Error(err.Error())
		//return err
	}

	var metrics []VMetric

	for _, pool := range pools {
		if pool.Summary != nil {
			// Get Cluster name from Resource Pool Parent
			cluster, err := ClusterFromID(c, pool.Parent.Value)
			if err != nil {
				log.Info(err.Error())
				return nil
			}

			// Get Quickstats form Resource Pool
			qs := pool.Summary.GetResourcePoolSummary().QuickStats

			// Memory
			metrics = append(metrics, VMetric{name: "vsphere_cluster_mem_ballooned", help: "The size of the balloon driver in a virtual machine, in MB. ", value: float64(qs.BalloonedMemory), labels: map[string]string{"cluster": cluster.Name(), "pool": pool.Name}})
			metrics = append(metrics, VMetric{name: "vsphere_cluster_mem_compressed", help: "The amount of compressed memory currently consumed by VM, in KB", value: float64(qs.CompressedMemory), labels: map[string]string{"cluster": cluster.Name(), "pool": pool.Name}})
			metrics = append(metrics, VMetric{name: "vsphere_cluster_mem_consumedOverhead", help: "The amount of overhead memory, in MB, currently being consumed to run a VM.", value: float64(qs.ConsumedOverheadMemory), labels: map[string]string{"cluster": cluster.Name(), "pool": pool.Name}})
			metrics = append(metrics, VMetric{name: "vsphere_cluster_mem_distributedMemoryEntitlement", help: "This is the amount of CPU resource, in MHz, that this VM is entitled to, as calculated by DRS.", value: float64(qs.DistributedMemoryEntitlement), labels: map[string]string{"cluster": cluster.Name(), "pool": pool.Name}})
			metrics = append(metrics, VMetric{name: "vsphere_cluster_mem_guest", help: "Guest memory utilization statistics, in MB. This is also known as active guest memory.", value: float64(qs.GuestMemoryUsage), labels: map[string]string{"cluster": cluster.Name(), "pool": pool.Name}})
			metrics = append(metrics, VMetric{name: "vsphere_cluster_mem_private", help: "The portion of memory, in MB, that is granted to a virtual machine from non-shared host memory.", value: float64(qs.PrivateMemory), labels: map[string]string{"cluster": cluster.Name(), "pool": pool.Name}})
			metrics = append(metrics, VMetric{name: "vsphere_cluster_mem_staticMemoryEntitlement", help: "The static memory resource entitlement for a virtual machine, in MB.", value: float64(qs.StaticMemoryEntitlement), labels: map[string]string{"cluster": cluster.Name(), "pool": pool.Name}})
			metrics = append(metrics, VMetric{name: "vsphere_cluster_mem_shared", help: "The portion of memory, in MB, that is granted to a virtual machine from host memory that is shared between VMs.", value: float64(qs.SharedMemory), labels: map[string]string{"cluster": cluster.Name(), "pool": pool.Name}})
			metrics = append(metrics, VMetric{name: "vsphere_cluster_mem_swapped", help: "The portion of memory, in MB, that is granted to a virtual machine from the host's swap space.", value: float64(qs.SwappedMemory), labels: map[string]string{"cluster": cluster.Name(), "pool": pool.Name}})
			metrics = append(metrics, VMetric{name: "vsphere_cluster_mem_limit", help: "Cluster Memory, in MB", value: float64(*pool.Config.MemoryAllocation.Limit), labels: map[string]string{"cluster": cluster.Name(), "pool": pool.Name}})
			metrics = append(metrics, VMetric{name: "vsphere_cluster_mem_usage", help: "Host memory utilization statistics, in MB. This is also known as consumed host memory.", value: float64(qs.HostMemoryUsage), labels: map[string]string{"cluster": cluster.Name(), "pool": pool.Name}})
			metrics = append(metrics, VMetric{name: "vsphere_cluster_mem_overhead", help: "The amount of memory resource (in MB) that will be used by a virtual machine above its guest memory requirements.", value: float64(qs.OverheadMemory), labels: map[string]string{"cluster": cluster.Name(), "pool": pool.Name}})

			// CPU
			metrics = append(metrics, VMetric{name: "vsphere_cluster_cpu_distributedCpuEntitlement", help: "This is the amount of CPU resource, in MHz, that this VM is entitled to.", value: float64(qs.DistributedCpuEntitlement), labels: map[string]string{"cluster": cluster.Name(), "pool": pool.Name}})
			metrics = append(metrics, VMetric{name: "vsphere_cluster_cpu_demand", help: "Basic CPU performance statistics, in MHz.", value: float64(qs.OverallCpuDemand), labels: map[string]string{"cluster": cluster.Name(), "pool": pool.Name}})
			metrics = append(metrics, VMetric{name: "vsphere_cluster_cpu_usage", help: "Basic CPU performance statistics, in MHz.", value: float64(qs.OverallCpuUsage), labels: map[string]string{"cluster": cluster.Name(), "pool": pool.Name}})
			metrics = append(metrics, VMetric{name: "vsphere_cluster_cpu_staticCpuEntitlement", help: "The static CPU resource entitlement for a virtual machine, in MHz.", value: float64(qs.StaticCpuEntitlement), labels: map[string]string{"cluster": cluster.Name(), "pool": pool.Name}})
			metrics = append(metrics, VMetric{name: "vsphere_cluster_cpu_limit", help: "Cluster CPU, MHz", value: float64(*pool.Config.CpuAllocation.Limit), labels: map[string]string{"cluster": cluster.Name(), "pool": pool.Name}})
		}
	}

	waitGroupCLS := sync.WaitGroup{}

	for _, objCLS := range arrCLS {
		if objCLS.Summary != nil {
			qs := objCLS.Summary.GetComputeResourceSummary()

			// Memory
			metrics = append(metrics, VMetric{name: "vsphere_cluster_mem_effective", help: "Effective memory resources available to run virtual machines, in MB.", value: float64(qs.EffectiveMemory), labels: map[string]string{"cluster": objCLS.Name}})
			metrics = append(metrics, VMetric{name: "vsphere_cluster_mem_total", help: "Aggregated memory resources of all hosts, in MB.", value: float64(qs.TotalMemory / 1024 / 1024), labels: map[string]string{"cluster": objCLS.Name}})

			// CPU
			metrics = append(metrics, VMetric{name: "vsphere_cluster_cpu_effective", help: "Effective CPU resources available to run virtual machines, in MHz.", value: float64(qs.EffectiveCpu), labels: map[string]string{"cluster": objCLS.Name}})
			metrics = append(metrics, VMetric{name: "vsphere_cluster_cpu_total", help: "Aggregated CPU resources of all hosts, in MHz.", value: float64(qs.TotalCpu), labels: map[string]string{"cluster": objCLS.Name}})
			metrics = append(metrics, VMetric{name: "vsphere_cluster_cpu_threads", help: "Aggregated number of CPU threads.", value: float64(qs.NumCpuThreads), labels: map[string]string{"cluster": objCLS.Name}})
			metrics = append(metrics, VMetric{name: "vsphere_cluster_cpu_cores", help: "Number of physical CPU cores. Physical CPU cores are the processors contained by a CPU package.", value: float64(qs.NumCpuCores), labels: map[string]string{"cluster": objCLS.Name}})

			// Misc
			metrics = append(metrics, VMetric{name: "vsphere_cluster_hosts_effective", help: "Total number of effective hosts.", value: float64(qs.NumEffectiveHosts), labels: map[string]string{"cluster": objCLS.Name}})
			metrics = append(metrics, VMetric{name: "vsphere_cluster_hosts_total", help: "Total number of hosts.", value: float64(qs.NumHosts), labels: map[string]string{"cluster": objCLS.Name}})

			waitGroupCLS.Add(1)
			go func(ch chan<- prometheus.Metric, objCLS mo.ClusterComputeResource) {
				defer waitGroupCLS.Done()
				defer timeTrack(ch, time.Now(), fmt.Sprintf("HostMetrics - %s", objCLS.Name))

				stats := HostMetrics(ch, objCLS)
				for _, s := range stats {
					metrics = append(metrics, s)
				}
			}(ch, objCLS)

		}
	}

	waitGroupCLS.Wait()

	return metrics
}

// HostMetrics Collects Hypervisor metrics
func HostMetrics(ch chan<- prometheus.Metric, objCLS mo.ClusterComputeResource) []VMetric {
	log.SetReportCaller(true)
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	c, err := NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	defer c.Logout(ctx)

	m := view.NewManager(c.Client)

	view, err := m.CreateContainerView(ctx, objCLS.Reference(), []string{"HostSystem"}, true)
	if err != nil {
		log.Error(err.Error())
	}

	defer view.Destroy(ctx)

	var hosts []mo.HostSystem
	err = view.Retrieve(ctx, []string{"HostSystem"}, []string{"summary", "parent", "vm"}, &hosts)
	if err != nil {
		log.Error(err.Error())
	}

	var metrics []VMetric

	for _, hs := range hosts {
		// Get name of cluster the host is part of
		cls, err := ClusterFromRef(c, hs.Parent.Reference())
		if err != nil {
			log.Error(err.Error())
			return nil
		}
		cname := cls.Name()
		cname = strings.ToLower(cname)

		name := hs.Summary.Config.Name
		totalCPU := int64(hs.Summary.Hardware.CpuMhz) * int64(hs.Summary.Hardware.NumCpuCores)
		freeCPU := int64(totalCPU) - int64(hs.Summary.QuickStats.OverallCpuUsage)
		cpuPusage := math.Round((float64(hs.Summary.QuickStats.OverallCpuUsage) / float64(totalCPU)) * 100)

		totalMemory := float64(hs.Summary.Hardware.MemorySize / 1024 / 1024 / 1024)
		usedMemory := float64(hs.Summary.QuickStats.OverallMemoryUsage / 1024)
		freeMemory := totalMemory - usedMemory
		memPusage := math.Round((usedMemory / totalMemory) * 100)

		metrics = append(metrics, VMetric{name: "vsphere_host_cpu_usage", help: "Hypervisors CPU usage", value: float64(hs.Summary.QuickStats.OverallCpuUsage), labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, VMetric{name: "vsphere_host_cpu_total", help: "Hypervisors CPU Total", value: float64(totalCPU), labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, VMetric{name: "vsphere_host_cpu_free", help: "Hypervisors CPU Free", value: float64(freeCPU), labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, VMetric{name: "vsphere_host_cpu_usage_percent", help: "Hypervisors CPU Percent Usage", value: float64(cpuPusage), labels: map[string]string{"host": name, "cluster": cname}})

		metrics = append(metrics, VMetric{name: "vsphere_host_mem_usage", help: "Hypervisors Memory Usage", value: usedMemory, labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, VMetric{name: "vsphere_host_mem_total", help: "Hypervisors Memory Total", value: totalMemory, labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, VMetric{name: "vsphere_host_mem_free", help: "Hypervisors Memory Free", value: float64(freeMemory), labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, VMetric{name: "vsphere_host_mem_usage_percent", help: "Hypervisors Memory Percent Usage", value: float64(memPusage), labels: map[string]string{"host": name, "cluster": cname}})

	}

	return metrics
}

// HostHBAStatus Report status of the HBA attached to a hypervisor to be able to monitor if a hba goes offline
func HostHBAStatus() []VMetric {
	log.SetReportCaller(true)
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	c, err := NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	defer c.Logout(ctx)

	m := view.NewManager(c.Client)

	view, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"HostSystem"}, true)
	if err != nil {
		log.Error(err.Error())
	}

	defer view.Destroy(ctx)

	var hosts []mo.HostSystem
	err = view.Retrieve(ctx, []string{"HostSystem"}, []string{"name", "parent"}, &hosts)
	if err != nil {
		log.Error(err.Error())
	}

	var metrics []VMetric

	for _, host := range hosts {
		// Get name of cluster the host is part of
		cls, err := ClusterFromRef(c, host.Parent.Reference())
		if err != nil {
			log.Info(err.Error())
			return nil
		}
		cname := cls.Name()
		cname = strings.ToLower(cname)

		hcm := object.NewHostConfigManager(c.Client, host.Reference())
		ss, err := hcm.StorageSystem(ctx)
		if err != nil {
			log.Error(err.Error())
		}

		var hss mo.HostStorageSystem
		err = ss.Properties(ctx, ss.Reference(), []string{"StorageDeviceInfo.HostBusAdapter"}, &hss)
		if err != nil {
			return nil
		}

		hbas := hss.StorageDeviceInfo.HostBusAdapter

		for _, v := range hbas {

			hba := v.GetHostHostBusAdapter()

			if hba.Status != "unknown" {
				status := 0
				if hba.Status == "online" {
					status = 1
				}
				metrics = append(metrics, VMetric{name: "vsphere_host_hba_status", help: "Hypervisors hba Online status, 1 == Online", value: float64(status), labels: map[string]string{"host": host.Name, "cluster": cname, "hba": hba.Device}})
			}

		}
	}

	return metrics
}

// VMMetrics TODO Comment
func VMMetrics() []VMetric {
	log.SetReportCaller(true)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	defer c.Logout(ctx)

	m := view.NewManager(c.Client)

	view, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
	if err != nil {
		log.Error(err.Error())
	}

	defer view.Destroy(ctx)

	var vms []mo.VirtualMachine

	// https://pubs.vmware.com/vi3/sdk/ReferenceGuide/vim.VirtualMachine.html#field_detail
	err = view.Retrieve(ctx, []string{"VirtualMachine"}, []string{"summary", "config", "name", "runtime", "guestHeartbeatStatus"}, &vms)
	if err != nil {
		log.Error(err.Error())
	}

	metricMap := GetMetricMap(ctx, c)

	idToName := make(map[int32]string)
	for k, v := range metricMap {
		idToName[v] = k
	}

	var metrics []VMetric

	for _, vm := range vms {
		// Labels - host, cluster
		host, cluster, err := GetVMLineage(ctx, c, vm.Runtime.Host.Reference())
		if err != nil {
			log.Error(err.Error())
			return nil
		}

		// Calculations
		freeMemory := (int64(vm.Summary.Config.MemorySizeMB)) - (int64(vm.Summary.QuickStats.GuestMemoryUsage))

		status := -1
		switch string(vm.GuestHeartbeatStatus) {
		case "green":
			status = 0
		case "yellow":
			status = 1
		case "red":
			status = 2
		}

		powerState := 0

		switch string(vm.Runtime.PowerState) {
		case "poweredOn":
			powerState = 1
		case "suspended":
			powerState = -1
		}

		// Add Metrics
		metrics = append(metrics, VMetric{name: "vsphere_vm_mem_total", help: "Memory size of the virtual machine, in MB.", value: float64(vm.Config.Hardware.MemoryMB), labels: map[string]string{"vm": vm.Name, "host": host.Name, "cluster": cluster.Name}})
		metrics = append(metrics, VMetric{name: "vsphere_vm_mem_free", help: "Guest memory free statistics, in MB. This is also known as free guest memory. The number can be between 0 and the configured memory size of the virtual machine. Valid while the virtual machine is running.", value: float64(freeMemory), labels: map[string]string{"vm": vm.Name, "host": host.Name, "cluster": cluster.Name}})
		metrics = append(metrics, VMetric{name: "vsphere_vm_mem_usage", help: "Guest memory utilization statistics, in MB. This is also known as active guest memory. The number can be between 0 and the configured memory size of the virtual machine. Valid while the virtual machine is running.", value: float64(vm.Summary.QuickStats.GuestMemoryUsage), labels: map[string]string{"vm": vm.Name, "host": host.Name, "cluster": cluster.Name}})

		metrics = append(metrics, VMetric{name: "vsphere_vm_cpu_usage", help: "Basic CPU performance statistics, in MHz. Valid while the virtual machine is running.", value: float64(vm.Summary.QuickStats.OverallCpuUsage), labels: map[string]string{"vm": vm.Name, "host": host.Name, "cluster": cluster.Name}})
		metrics = append(metrics, VMetric{name: "vsphere_vm_cpu_count", help: "Number of processors in the virtual machine.", value: float64(vm.Summary.Config.NumCpu), labels: map[string]string{"vm": vm.Name, "host": host.Name, "cluster": cluster.Name}})

		metrics = append(metrics, VMetric{name: "vsphere_vm_heartbeat", help: "Overall alarm status on this node from VMware Tools.", value: float64(status), labels: map[string]string{"vm": vm.Name, "host": host.Name, "cluster": cluster.Name}})
		metrics = append(metrics, VMetric{name: "vsphere_vm_powerstate", help: "The current power state of the virtual machine.", value: float64(powerState), labels: map[string]string{"vm": vm.Name, "host": host.Name, "cluster": cluster.Name}})

	}

	return metrics
}

// GetClusters TODO Comment
func GetClusters(ctx context.Context, c *govmomi.Client, lst *[]mo.ClusterComputeResource) error {
	m := view.NewManager(c.Client)

	view, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"ClusterComputeResource"}, true)
	if err != nil {
		log.Error(err.Error())
		return err
	}

	defer view.Destroy(ctx)

	err = view.Retrieve(ctx, []string{"ClusterComputeResource"}, []string{"name", "summary"}, lst)
	if err != nil {
		log.Error(err.Error())
		return err
	}

	return nil
}

// ClusterFromID returns a ClusterComputeResource, a subclass of
// ComputeResource that is used for clusters.
func ClusterFromID(client *govmomi.Client, id string) (*object.ClusterComputeResource, error) {
	finder := find.NewFinder(client.Client, false)

	ref := types.ManagedObjectReference{
		Type:  "ClusterComputeResource",
		Value: id,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	obj, err := finder.ObjectReference(ctx, ref)
	if err != nil {
		return nil, err
	}
	return obj.(*object.ClusterComputeResource), nil
}

// ClusterFromRef TODO Comment
func ClusterFromRef(client *govmomi.Client, ref types.ManagedObjectReference) (*object.ClusterComputeResource, error) {
	finder := find.NewFinder(client.Client, false)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	obj, err := finder.ObjectReference(ctx, ref)
	if err != nil {
		return nil, err
	}
	typeObj := reflect.TypeOf(obj)
	switch typeObj.String() {
	case "*object.ClusterComputeResource":
		return obj.(*object.ClusterComputeResource), nil
	case "*object.ComputeResource":
		return nil, errors.New("ClusterFromRef is connected locally to a EXSi host, not a vSphere")
	}
	return nil, errors.New("ClusterFromRef returned an unknown type, please create an issue")
}

// GetVMLineage gets the parent and grandparent ManagedEntity objects
func GetVMLineage(ctx context.Context, client *govmomi.Client, host types.ManagedObjectReference) (mo.ManagedEntity, mo.ManagedEntity, error) {
	var hostEntity mo.ManagedEntity
	err := client.RetrieveOne(ctx, host.Reference(), []string{"name", "parent"}, &hostEntity)
	if err != nil {
		log.Fatal(err)
	}

	var clusterEntity mo.ManagedEntity
	err = client.RetrieveOne(ctx, hostEntity.Parent.Reference(), []string{"name", "parent"}, &clusterEntity)
	if err != nil {
		log.Fatal(err)
	}

	return hostEntity, clusterEntity, nil
}

// GetMetricMap TODO Comment
func GetMetricMap(ctx context.Context, client *govmomi.Client) (MetricMap map[string]int32) {

	var pM mo.PerformanceManager
	err := client.RetrieveOne(ctx, *client.ServiceContent.PerfManager, nil, &pM)
	if err != nil {
		log.Fatal(err)
	}

	metricMap := make(map[string]int32)

	for _, perfCounterInfo := range pM.PerfCounter {
		name := perfCounterInfo.GroupInfo.GetElementDescription().Key + "." + perfCounterInfo.NameInfo.GetElementDescription().Key + "." + string(perfCounterInfo.RollupType)
		metricMap[name] = perfCounterInfo.Key
	}
	return metricMap
}

// PerfQuery TODO Comment
func PerfQuery(ctx context.Context, c *govmomi.Client, metrics []string, entity mo.ManagedEntity, nameToID map[string]int32, idToName map[int32]string) map[string]int64 {

	var pM mo.PerformanceManager
	err := c.RetrieveOne(ctx, *c.ServiceContent.PerfManager, nil, &pM)
	if err != nil {
		log.Fatal(err)
	}

	var pmidList []types.PerfMetricId
	for _, v := range metrics {
		mid := types.PerfMetricId{CounterId: nameToID[v]}
		pmidList = append(pmidList, mid)
	}

	querySpec := types.PerfQuerySpec{
		Entity:     entity.Reference(),
		MetricId:   pmidList,
		MaxSample:  3,
		IntervalId: 20,
	}
	query := types.QueryPerf{
		This:      pM.Reference(),
		QuerySpec: []types.PerfQuerySpec{querySpec},
	}

	response, err := methods.QueryPerf(ctx, c, &query)
	if err != nil {
		log.Fatal(err)
	}

	data := make(map[string]int64)
	for _, base := range response.Returnval {
		metric := base.(*types.PerfEntityMetric)
		for _, baseSeries := range metric.Value {
			series := baseSeries.(*types.PerfMetricIntSeries)
			//fmt.Print(idToName[series.Id.CounterId] + ": ")
			var sum int64
			for _, v := range series.Value {
				sum = sum + v
			}
			data[idToName[series.Id.CounterId]] = sum / 3
		}
	}
	return data
}

/*
// ClusterCounters TODO Comment
func ClusterCounters() []VMetric {
	log.SetReportCaller(true)
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	c, err := NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	defer c.Logout(ctx)

	m := view.NewManager(c.Client)

	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"ClusterComputeResource"}, true)
	if err != nil {
		log.Error(err.Error())

	}

	defer v.Destroy(ctx)

	var lst []mo.ClusterComputeResource
	err = v.Retrieve(ctx, []string{"ClusterComputeResource"}, []string{"name"}, &lst)
	if err != nil {
		log.Error(err.Error())

	}

	pm := performance.NewManager(c.Client)
	mlist, err := pm.CounterInfoByKey(ctx)
	if err != nil {
		log.Error(err.Error())

	}

	var metrics []VMetric

	for _, cls := range lst {
		cname := cls.Name
		cname = strings.ToLower(cname)

		am, _ := pm.AvailableMetric(ctx, cls.Reference(), 300)

		var pqList []types.PerfMetricId
		for _, v := range am {

			if strings.Contains(mlist[v.CounterId].Name(), "vmop") {
				pqList = append(pqList, v)
			}
		}

		querySpec := types.PerfQuerySpec{
			Entity:     cls.Reference(),
			MetricId:   pqList,
			MaxSample:  1,
			IntervalId: 300,
		}
		query := types.QueryPerf{
			This:      pm.Reference(),
			QuerySpec: []types.PerfQuerySpec{querySpec},
		}

		response, err := methods.QueryPerf(ctx, c, &query)
		if err != nil {
			log.Fatal(err)
		}

		// vsphere_cluster_vmop_numChangeDS{cluster="ucs"} 1
		// vsphere_cluster_vmop_numChangeHo{cluster="ucs"} 8
		// vsphere_cluster_vmop_numChangeHostDS{cluster="ucs"} 0
		// vsphere_cluster_vmop_numClon{cluster="ucs"} 0
		// vsphere_cluster_vmop_numCr{cluster="ucs"} 3
		// vsphere_cluster_vmop_numDeploy{cluster="ucs"} 0
		// vsphere_cluster_vmop_numDestroy{cluster="ucs"} 4
		// vsphere_cluster_vmop_numPoweroff{cluster="ucs"} 3
		// vsphere_cluster_vmop_numPoweron{cluster="ucs"} 11
		// vsphere_cluster_vmop_numR{cluster="ucs"} 1
		// vsphere_cluster_vmop_numRebootGu{cluster="ucs"} 0
		// vsphere_cluster_vmop_numReconfigur{cluster="ucs"} 100
		// vsphere_cluster_vmop_numRegister{cluster="ucs"} 0
		// vsphere_cluster_vmop_numSVMotion{cluster="ucs"} 3
		// vsphere_cluster_vmop_numShutdownGu{cluster="ucs"} 1
		// vsphere_cluster_vmop_numStandbyGu{cluster="ucs"} 0
		// vsphere_cluster_vmop_numSuspend{cluster="ucs"} 0
		// vsphere_cluster_vmop_numUnregister{cluster="ucs"} 0
		// vsphere_cluster_vmop_numVMotion{cluster="ucs"} 10
		// vsphere_cluster_vmop_numXVMotion{cluster="ucs"} 0

		for _, base := range response.Returnval {
			metric := base.(*types.PerfEntityMetric)
			for _, baseSeries := range metric.Value {
				series := baseSeries.(*types.PerfMetricIntSeries)
				name := strings.TrimLeft(mlist[series.Id.CounterId].Name(), "vmop.")
				name = strings.TrimRight(name, ".latest")
				metrics = append(metrics, VMetric{name: "vsphere_cluster_vmop_" + name, help: "vmops counter ", value: float64(series.Value[0]), labels: map[string]string{"cluster": cname}})

			}
		}

	}
	return metrics
}
*/

/*
// HostCounters Collects Hypervisor counters
func HostCounters() []VMetric {
	log.SetReportCaller(true)
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	c, err := NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	defer c.Logout(ctx)

	m := view.NewManager(c.Client)

	view, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"HostSystem"}, true)
	if err != nil {
		log.Error(err.Error() + ": HostCounters")
	}

	defer view.Destroy(ctx)

	var hosts []mo.HostSystem
	err = view.Retrieve(ctx, []string{"HostSystem"}, []string{"name", "parent", "summary"}, &hosts)
	if err != nil {
		log.Error(err.Error() + ": HostCounters")
	}

	var metrics []VMetric

	for _, hs := range hosts {
		// Get name of cluster the host is part of
		cls, err := ClusterFromRef(c, hs.Parent.Reference())
		if err != nil {
			log.Error(err.Error())
			return nil
		}
		cname := cls.Name()
		cname = strings.ToLower(cname)
		name := hs.Summary.Config.Name

		vMgr := view.NewManager(c.Client)
		vmView, err := vMgr.CreateContainerView(ctx, hs.Reference(), []string{"VirtualMachine"}, true)
		if err != nil {
			log.Error(err.Error() + " " + hs.Name)
		}

		var vms []mo.VirtualMachine

		err2 := vmView.RetrieveWithFilter(ctx, []string{"VirtualMachine"}, []string{"name", "runtime"}, &vms, property.Filter{"runtime.powerState": "poweredOn"})
		if err2 != nil {
			//	log.Error(err2.Error() +": HostCounters - poweron")
		}

		poweredOn := len(vms)

		err = vmView.Retrieve(ctx, []string{"VirtualMachine"}, []string{"name", "summary.config", "runtime.powerState"}, &vms)
		if err != nil {
			log.Error(err.Error() + " : " + "in retrieving vms")
		}

		total := len(vms)

		metrics = append(metrics, VMetric{name: "vsphere_host_vm_poweron", help: "Number of vms running on host", value: float64(poweredOn), labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, VMetric{name: "vsphere_host_vm_total", help: "Number of vms registered on host", value: float64(total), labels: map[string]string{"host": name, "cluster": cname}})

		var vMem int64
		var vCPU int64
		var vCPUOn int64
		var vMemOn int64
		vCPU = 0
		vMem = 0
		vCPUOn = 0
		vMemOn = 0

		for _, vm := range vms {

			vCPU = vCPU + int64(vm.Summary.Config.NumCpu)
			vMem = vMem + int64(vm.Summary.Config.MemorySizeMB/1024)

			pwr := string(vm.Runtime.PowerState)
			//fmt.Println(pwr)
			if pwr == "poweredOn" {
				vCPUOn = vCPUOn + int64(vm.Summary.Config.NumCpu)
				vMemOn = vMemOn + int64(vm.Summary.Config.MemorySizeMB/1024)
			}
		}

		metrics = append(metrics, VMetric{name: "vsphere_host_vcpu_all", help: "Number of vcpu configured on host", value: float64(vCPU), labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, VMetric{name: "vsphere_host_vmem_all", help: "Total vmem configured on host", value: float64(vMem), labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, VMetric{name: "vsphere_host_vcpu_on", help: "Number of vcpu configured and running on host", value: float64(vCPUOn), labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, VMetric{name: "vsphere_host_vmem_on", help: "Total vmem configured and running on host", value: float64(vMemOn), labels: map[string]string{"host": name, "cluster": cname}})

		cores := hs.Summary.Hardware.NumCpuCores
		metrics = append(metrics, VMetric{name: "vsphere_host_cores", help: "Number of physical cores available on host", value: float64(cores), labels: map[string]string{"host": name, "cluster": cname}})

		vmView.Destroy(ctx)
	}

	return metrics
}
*/
