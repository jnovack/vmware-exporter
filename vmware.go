package main

import (
	"context"
	log "github.com/Sirupsen/logrus"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c, err := NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	defer c.Logout(ctx)

	m := view.NewManager(c.Client)

	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"Datastore"}, true)
	if err != nil {
		log.Error(err.Error())

	}

	defer v.Destroy(ctx)

	var dsl []mo.Datastore
	err = v.Retrieve(ctx, []string{"Datastore"}, []string{"summary", "name", "parent"}, &dsl)
	if err != nil {
		log.Error(err.Error())
	}

	var metrics []vMetric

	for _, ds := range dsl {
		if ds.Summary.Accessible {

			ds_capacity := ds.Summary.Capacity / 1024 / 1024 / 1024
			ds_freespace := ds.Summary.FreeSpace / 1024 / 1024 / 1024
			ds_used := ds_capacity - ds_freespace
			ds_pused := math.Round((float64(ds_used) / float64(ds_capacity)) * 100)
			ds_uncommitted := ds.Summary.Uncommitted / 1024 / 1024 / 1024
			ds_name := ds.Summary.Name

			metrics = append(metrics, vMetric{name: "vsphere_datastore_capacity_size", help: "Datastore Total Size", value: float64(ds_capacity), labels: map[string]string{"datastore": ds_name}})
			metrics = append(metrics, vMetric{name: "vsphere_datastore_capacity_free", help: "Datastore Size Free", value: float64(ds_freespace), labels: map[string]string{"datastore": ds_name}})
			metrics = append(metrics, vMetric{name: "vsphere_datastore_capacity_used", help: "Datastore Size Used", value: float64(ds_used), labels: map[string]string{"datastore": ds_name}})
			metrics = append(metrics, vMetric{name: "vsphere_datastore_capacity_uncommitted", help: "Datastore Size Uncommitted", value: float64(ds_uncommitted), labels: map[string]string{"datastore": ds_name}})
			metrics = append(metrics, vMetric{name: "vsphere_datastore_capacity_pused", help: "Datastore Size", value: ds_pused, labels: map[string]string{"datastore": ds_name}})

		}

	}

	return metrics
}

func ClusterMetrics() []vMetric {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
		//return err
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
			metrics = append(metrics, vMetric{name: "vsphere_cluster_cpu_effective", help: "Effective available CPU MHz in Cluster", value: float64(qs.EffectiveCpu), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "vsphere_cluster_cpu_total", help: "Total Amount of CPU MHz in Cluster", value: float64(qs.TotalCpu), labels: map[string]string{"cluster": cname}})

			// Misc
			metrics = append(metrics, vMetric{name: "vsphere_cluster_numHosts", help: "Number of Hypervisors in cluster", value: float64(qs.NumHosts), labels: map[string]string{"cluster": cname}})

			//ccrs := qs.su.(*types.ClusterComputeResourceSummary)
			// TODO fix missing ClusterComputeResourceSummary no only BaseComputeResourceSummary available
			//metrics = append(metrics, vMetric{ name: "cluster_numVmotions", help: "Cluster Number of vmotions ", value: float64(qs.NumHosts), labels: map[string]string{"cluster": cname}})

			// Check VMS powered vs created in cluster
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

func HostMetrics() []vMetric {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
	err = v.Retrieve(ctx, []string{"HostSystem"}, []string{"summary", "parent"}, &hosts)
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

func VmMetrics() []vMetric {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	err = v.Retrieve(ctx, []string{"VirtualMachine"}, []string{"summary", "name"}, &vms)
	if err != nil {
		log.Error(err.Error())
	}

	var metrics []vMetric

	for _, vm := range vms {

		BalloonedMemory := vm.Summary.QuickStats.BalloonedMemory
		GuestMemoryUsage := vm.Summary.QuickStats.GuestMemoryUsage

		metrics = append(metrics, vMetric{name: "vsphere_vm_mem_total", help: "VM Memory total", value: float64(vm.Config.Hardware.MemoryMB), labels: map[string]string{"vmname": vm.Name}})
		metrics = append(metrics, vMetric{name: "vsphere_vm_mem_usage", help: "VM Memory usage", value: float64(GuestMemoryUsage), labels: map[string]string{"vmname": vm.Name}})
		metrics = append(metrics, vMetric{name: "vsphere_vm_mem_balloonede", help: "VM Memory Ballooned", value: float64(BalloonedMemory), labels: map[string]string{"vmname": vm.Name}})

		metrics = append(metrics, vMetric{name: "vsphere_vm_cpu_usage", help: "VM CPU Usage", value: float64(vm.Summary.QuickStats.OverallCpuUsage), labels: map[string]string{"vmname": vm.Name}})
		metrics = append(metrics, vMetric{name: "vsphere_vm_cpu_demand", help: "VM CPU Demand", value: float64(vm.Summary.QuickStats.OverallCpuDemand), labels: map[string]string{"vmname": vm.Name}})

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
