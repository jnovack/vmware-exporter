package main

import (
	"context"
	log "github.com/Sirupsen/logrus"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/performance"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"math"
	"net/url"
	"strings"
	"time"
)

type vMetric struct {
	name   string
	help   string
	value  float64
	labels map[string]string
}

// Connect to vCenter
func NewClient(ctx context.Context) (*govmomi.Client, error) {

	u, err := url.Parse("https://" + cfg.Host + vim25.Path)
	if err != nil {
		log.Fatal(err)
	}
	u.User = url.UserPassword(cfg.User, cfg.Password)
	log.Debug("Connecting to " + u.String())

	return govmomi.NewClient(ctx, u, true)
}

func DSMetrics() []vMetric {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	c, err := NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	defer c.Logout(ctx)

	m := view.NewManager(c.Client)

	vmgr, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"ClusterComputeResource"}, true)
	if err != nil {
		log.Error(err.Error())

	}

	defer vmgr.Destroy(ctx)

	var metrics []vMetric

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

				ds_capacity := ds.Summary.Capacity / 1024 / 1024 / 1024
				ds_freespace := ds.Summary.FreeSpace / 1024 / 1024 / 1024
				ds_used := ds_capacity - ds_freespace
				ds_pused := math.Round((float64(ds_used) / float64(ds_capacity)) * 100)
				ds_uncommitted := ds.Summary.Uncommitted / 1024 / 1024 / 1024
				ds_name := ds.Summary.Name

				metrics = append(metrics, vMetric{name: "vsphere_datastore_capacity_size", help: "Datastore Total Size", value: float64(ds_capacity), labels: map[string]string{"datastore": ds_name, "cluster": cname}})
				metrics = append(metrics, vMetric{name: "vsphere_datastore_capacity_free", help: "Datastore Size Free", value: float64(ds_freespace), labels: map[string]string{"datastore": ds_name, "cluster": cname}})
				metrics = append(metrics, vMetric{name: "vsphere_datastore_capacity_used", help: "Datastore Size Used", value: float64(ds_used), labels: map[string]string{"datastore": ds_name, "cluster": cname}})
				metrics = append(metrics, vMetric{name: "vsphere_datastore_capacity_uncommitted", help: "Datastore Size Uncommitted", value: float64(ds_uncommitted), labels: map[string]string{"datastore": ds_name, "cluster": cname}})
				metrics = append(metrics, vMetric{name: "vsphere_datastore_capacity_pused", help: "Datastore Size", value: ds_pused, labels: map[string]string{"datastore": ds_name, "cluster": cname}})
			}

		}
	}

	return metrics
}

func ClusterMetrics() []vMetric {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	c, err := NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	defer c.Logout(ctx)

	var clusters []mo.ClusterComputeResource
	e2 := GetClusters(ctx, c, &clusters)
	if e2 != nil {
		log.Error(e2.Error())
	}

	m := view.NewManager(c.Client)

	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"ResourcePool"}, true)
	if err != nil {
		log.Error(err.Error())
	}

	defer v.Destroy(ctx)

	var pools []mo.ResourcePool
	err = v.RetrieveWithFilter(ctx, []string{"ResourcePool"}, []string{"summary", "name", "parent", "config"}, &pools, property.Filter{"name": "Resources"})
	if err != nil {
		log.Error(err.Error())
		//return err
	}

	var metrics []vMetric

	for _, pool := range pools {
		if pool.Summary != nil {
			// Get Cluster name from Resource Pool Parent
			cls, err := ClusterFromID(c, pool.Parent.Value)
			if err != nil {
				log.Error(err.Error())
				return nil
			}

			cname := cls.Name()
			cname = strings.ToLower(cname)

			// Get Quickstats form Resource Pool
			qs := pool.Summary.GetResourcePoolSummary().QuickStats
			memLimit := pool.Config.MemoryAllocation.Limit

			// Memory
			metrics = append(metrics, vMetric{name: "vsphere_cluster_mem_ballooned", help: "Cluster Memory Ballooned", value: float64(qs.BalloonedMemory / 1024), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "vsphere_cluster_mem_compressed", help: "The amount of compressed memory currently consumed by VM", value: float64(qs.CompressedMemory / 1024 / 1024), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "vsphere_cluster_mem_consumedOverhead", help: "The amount of overhead memory", value: float64(qs.ConsumedOverheadMemory / 1024), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "vsphere_cluster_mem_distributedMemoryEntitlement", help: "Cluster Memory ", value: float64(qs.DistributedMemoryEntitlement / 1024), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "vsphere_cluster_mem_guest", help: "Guest memory utilization statistics", value: float64(qs.GuestMemoryUsage / 1024), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "vsphere_cluster_mem_private", help: "Cluster Memory ", value: float64(qs.PrivateMemory / 1024), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "vsphere_cluster_mem_staticMemoryEntitlement", help: "Cluster Memory ", value: float64(qs.StaticMemoryEntitlement / 1024), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "vsphere_cluster_mem_shared", help: "Cluster Memory ", value: float64(qs.SharedMemory / 1024), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "vsphere_cluster_mem_swapped", help: "Cluster Memory ", value: float64(qs.SwappedMemory / 1024), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "vsphere_cluster_mem_limit", help: "Cluster Memory ", value: float64(*memLimit / 1024 / 1024), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "vsphere_cluster_mem_usage", help: "Cluster Memory ", value: float64(qs.HostMemoryUsage / 1024), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "vsphere_cluster_mem_overhead", help: "Cluster Memory ", value: float64(qs.OverheadMemory / 1024), labels: map[string]string{"cluster": cname}})

			// CPU
			metrics = append(metrics, vMetric{name: "vsphere_cluster_cpu_distributedCpuEntitlement", help: "Cluster CPU, MHz ", value: float64(qs.DistributedCpuEntitlement), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "vsphere_cluster_cpu_demand", help: "Cluster CPU demand, MHz", value: float64(qs.OverallCpuDemand), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "vsphere_cluster_cpu_usage", help: "Cluster CPU usage MHz", value: float64(qs.OverallCpuUsage), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "vsphere_cluster_cpu_staticCpuEntitlement", help: "Cluster CPU static, MHz", value: float64(qs.StaticCpuEntitlement), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "vsphere_cluster_cpu_limit", help: "Cluster CPU, MHz ", value: float64(*pool.Config.CpuAllocation.Limit), labels: map[string]string{"cluster": cname}})
		}
	}

	for _, cl := range clusters {
		if cl.Summary != nil {
			cname := cl.Name
			cname = strings.ToLower(cname)
			qs := cl.Summary.GetComputeResourceSummary()

			// Memory
			metrics = append(metrics, vMetric{name: "vsphere_cluster_mem_effective", help: "Effective amount of Memory in Cluster", value: float64(qs.EffectiveMemory / 1024), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "vsphere_cluster_mem_total", help: "Total Amount of Memory in Cluster", value: float64(qs.TotalMemory / 1024 / 1024 / 1024), labels: map[string]string{"cluster": cname}})

			// CPU
			metrics = append(metrics, vMetric{name: "vsphere_cluster_cpu_effective", help: "Effective available CPU Hz in Cluster", value: float64(qs.EffectiveCpu), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "vsphere_cluster_cpu_total", help: "Total Amount of CPU Hz in Cluster", value: float64(qs.TotalCpu), labels: map[string]string{"cluster": cname}})

			// Misc
			metrics = append(metrics, vMetric{name: "vsphere_cluster_numHosts", help: "Number of Hypervisors in cluster", value: float64(qs.NumHosts), labels: map[string]string{"cluster": cname}})

			// Virtual Servers, powered on vs created in cluster
			v, err := m.CreateContainerView(ctx, cl.Reference(), []string{"VirtualMachine"}, true)
			if err != nil {
				log.Error(err.Error())
			}

			var vms []mo.VirtualMachine

			err = v.RetrieveWithFilter(ctx, []string{"VirtualMachine"}, []string{"summary", "parent"}, &vms, property.Filter{"runtime.powerState": "poweredOn"})
			if err != nil {
				log.Error(err.Error())
			}
			poweredOn := len(vms)

			err = v.Retrieve(ctx, []string{"VirtualMachine"}, []string{"summary", "parent"}, &vms)
			if err != nil {
				log.Error(err.Error())
			}
			total := len(vms)

			metrics = append(metrics, vMetric{name: "vsphere_cluster_vm_poweredon", help: "Number of vms running in cluster", value: float64(poweredOn), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "vsphere_cluster_vm_total", help: "Number of vms in cluster", value: float64(total), labels: map[string]string{"cluster": cname}})

		}

	}

	return metrics
}

func ClusterCounters() []vMetric {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	var metrics []vMetric

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

		for _, base := range response.Returnval {
			metric := base.(*types.PerfEntityMetric)
			for _, baseSeries := range metric.Value {
				series := baseSeries.(*types.PerfMetricIntSeries)
				name := strings.TrimLeft(mlist[series.Id.CounterId].Name(), "vmop.")
				name = strings.TrimRight(name, ".latest")
				metrics = append(metrics, vMetric{name: "vsphere_cluster_vmop_" + name, help: "vmops counter ", value: float64(series.Value[0]), labels: map[string]string{"cluster": cname}})

			}
		}

	}
	return metrics
}

// Collects Hypervisor metrics
func HostMetrics() []vMetric {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	c, err := NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	defer c.Logout(ctx)

	m := view.NewManager(c.Client)

	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"HostSystem"}, true)
	if err != nil {
		log.Error(err.Error())
	}

	defer v.Destroy(ctx)

	var hosts []mo.HostSystem
	err = v.Retrieve(ctx, []string{"HostSystem"}, []string{"summary", "parent", "vm"}, &hosts)
	if err != nil {
		log.Error(err.Error())
	}

	var metrics []vMetric

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

		metrics = append(metrics, vMetric{name: "vsphere_host_cpu_usage", help: "Hypervisors CPU usage", value: float64(hs.Summary.QuickStats.OverallCpuUsage), labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, vMetric{name: "vsphere_host_cpu_total", help: "Hypervisors CPU Total", value: float64(totalCPU), labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, vMetric{name: "vsphere_host_cpu_free", help: "Hypervisors CPU Free", value: float64(freeCPU), labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, vMetric{name: "vsphere_host_cpu_pusage", help: "Hypervisors CPU Percent Usage", value: float64(cpuPusage), labels: map[string]string{"host": name, "cluster": cname}})

		metrics = append(metrics, vMetric{name: "vsphere_host_mem_usage", help: "Hypervisors Memory Usage", value: usedMemory, labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, vMetric{name: "vsphere_host_mem_total", help: "Hypervisors Memory Total", value: totalMemory, labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, vMetric{name: "vsphere_host_mem_free", help: "Hypervisors Memory Free", value: float64(freeMemory), labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, vMetric{name: "vsphere_host_mem_pusage", help: "Hypervisors Memory Percent Usage", value: float64(memPusage), labels: map[string]string{"host": name, "cluster": cname}})

	}

	return metrics
}

// Collects Hypervisor counters
func HostCounters() []vMetric {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	c, err := NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	defer c.Logout(ctx)

	m := view.NewManager(c.Client)

	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"HostSystem"}, true)
	if err != nil {
		log.Error(err.Error() + ": HostCounters")
	}

	defer v.Destroy(ctx)

	var hosts []mo.HostSystem
	err = v.Retrieve(ctx, []string{"HostSystem"}, []string{"name", "parent", "summary"}, &hosts)
	if err != nil {
		log.Error(err.Error() + ": HostCounters")
	}

	var metrics []vMetric

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

		metrics = append(metrics, vMetric{name: "vsphere_host_vm_poweron", help: "Number of vms running on host", value: float64(poweredOn), labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, vMetric{name: "vsphere_host_vm_total", help: "Number of vms registered on host", value: float64(total), labels: map[string]string{"host": name, "cluster": cname}})

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

		metrics = append(metrics, vMetric{name: "vsphere_host_vcpu_all", help: "Number of vcpu configured on host", value: float64(vCPU), labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, vMetric{name: "vsphere_host_vmem_all", help: "Total vmem configured on host", value: float64(vMem), labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, vMetric{name: "vsphere_host_vcpu_on", help: "Number of vcpu configured and running on host", value: float64(vCPUOn), labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, vMetric{name: "vsphere_host_vmem_on", help: "Total vmem configured and running on host", value: float64(vMemOn), labels: map[string]string{"host": name, "cluster": cname}})

		cores := hs.Summary.Hardware.NumCpuCores
		metrics = append(metrics, vMetric{name: "vsphere_host_cores", help: "Number of physical cores available on host", value: float64(cores), labels: map[string]string{"host": name, "cluster": cname}})

		vmView.Destroy(ctx)
	}

	return metrics
}

// Report status of the HBA attached to a hypervisor to be able to monitor if a hba goes offline
func HostHBAStatus() []vMetric {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	c, err := NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	defer c.Logout(ctx)

	m := view.NewManager(c.Client)

	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"HostSystem"}, true)
	if err != nil {
		log.Error(err.Error())
	}

	defer v.Destroy(ctx)

	var hosts []mo.HostSystem
	err = v.Retrieve(ctx, []string{"HostSystem"}, []string{"name", "parent"}, &hosts)
	if err != nil {
		log.Error(err.Error())
	}

	var metrics []vMetric

	for _, host := range hosts {
		// Get name of cluster the host is part of
		cls, err := ClusterFromRef(c, host.Parent.Reference())
		if err != nil {
			log.Error(err.Error())
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
				metrics = append(metrics, vMetric{name: "vsphere_host_hba_status", help: "Hypervisors hba Online status, 1 == Online", value: float64(status), labels: map[string]string{"host": host.Name, "cluster": cname, "hba": hba.Device}})
			}

		}
	}

	return metrics
}

func VmMetrics() []vMetric {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	defer c.Logout(ctx)

	m := view.NewManager(c.Client)

	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
	if err != nil {
		log.Error(err.Error())
	}

	defer v.Destroy(ctx)

	var vms []mo.VirtualMachine

	err = v.Retrieve(ctx, []string{"VirtualMachine"}, []string{"summary", "config", "name"}, &vms)
	if err != nil {
		log.Error(err.Error())
	}

	metricMap := GetMetricMap(ctx, c)

	idToName := make(map[int32]string)
	for k, v := range metricMap {
		idToName[v] = k
	}

	var metrics []vMetric

	for _, vm := range vms {

		// VM Memory
		freeMemory := (int64(vm.Summary.Config.MemorySizeMB) * 1024 * 1024) - (int64(vm.Summary.QuickStats.GuestMemoryUsage) * 1024 * 1024)
		BalloonedMemory := int64(vm.Summary.QuickStats.BalloonedMemory) * 1024 * 1024
		GuestMemoryUsage := int64(vm.Summary.QuickStats.GuestMemoryUsage) * 1024 * 1024
		VmMemory := int64(vm.Config.Hardware.MemoryMB) * 1024 * 1024

		metrics = append(metrics, vMetric{name: "vsphere_vm_mem_total", help: "VM Memory total, Byte", value: float64(VmMemory), labels: map[string]string{"vmname": vm.Name}})
		metrics = append(metrics, vMetric{name: "vsphere_vm_mem_free", help: "VM Memory total, Byte", value: float64(freeMemory), labels: map[string]string{"vmname": vm.Name}})
		metrics = append(metrics, vMetric{name: "vsphere_vm_mem_usage", help: "VM Memory usage, Byte", value: float64(GuestMemoryUsage), labels: map[string]string{"vmname": vm.Name}})
		metrics = append(metrics, vMetric{name: "vsphere_vm_mem_balloonede", help: "VM Memory Ballooned, Byte", value: float64(BalloonedMemory), labels: map[string]string{"vmname": vm.Name}})

		metrics = append(metrics, vMetric{name: "vsphere_vm_cpu_usage", help: "VM CPU Usage, MHz", value: float64(vm.Summary.QuickStats.OverallCpuUsage), labels: map[string]string{"vmname": vm.Name}})
		metrics = append(metrics, vMetric{name: "vsphere_vm_cpu_demand", help: "VM CPU Demand, MHz", value: float64(vm.Summary.QuickStats.OverallCpuDemand), labels: map[string]string{"vmname": vm.Name}})
		//metrics = append(metrics, vMetric{name: "vsphere_vm_cpu_total", help: "VM CPU Demand, MHz", value: float64(vm.Config), labels: map[string]string{"vmname": vm.Name}})
		/*
			perfMetrics := PerfQuery(ctx,c,[]string{"cpu.ready.summation", "cpu.usage.none"},vm.GetManagedEntity(),metricMap,idToName)
			metrics = append(metrics, vMetric{name: "vsphere_vm_cpu_ready", help: "VM CPU % Ready", value: float64(perfMetrics["cpu.ready.summation"]), labels: map[string]string{"vmname": vm.Name}})
		*/

	}

	return metrics
}

func GetClusters(ctx context.Context, c *govmomi.Client, lst *[]mo.ClusterComputeResource) error {

	m := view.NewManager(c.Client)

	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"ClusterComputeResource"}, true)
	if err != nil {
		log.Error(err.Error())
		return err
	}

	defer v.Destroy(ctx)

	err = v.Retrieve(ctx, []string{"ClusterComputeResource"}, []string{"name", "summary"}, lst)
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

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	obj, err := finder.ObjectReference(ctx, ref)
	if err != nil {
		return nil, err
	}
	return obj.(*object.ClusterComputeResource), nil
}

func ClusterFromRef(client *govmomi.Client, ref types.ManagedObjectReference) (*object.ClusterComputeResource, error) {
	finder := find.NewFinder(client.Client, false)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	obj, err := finder.ObjectReference(ctx, ref)
	if err != nil {
		return nil, err
	}
	return obj.(*object.ClusterComputeResource), nil
}

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
func PerfQuery(ctx context.Context, c *govmomi.Client, metrics []string, entity mo.ManagedEntity, nameToId map[string]int32, idToName map[int32]string) map[string]int64 {

	var pM mo.PerformanceManager
	err := c.RetrieveOne(ctx, *c.ServiceContent.PerfManager, nil, &pM)
	if err != nil {
		log.Fatal(err)
	}

	var pmidList []types.PerfMetricId
	for _, v := range metrics {
		mid := types.PerfMetricId{CounterId: nameToId[v]}
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
