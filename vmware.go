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
	"time"
)

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
	ctx := context.Background()

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
	err = v.Retrieve(ctx, []string{"Datastore"}, []string{"summary", "name"}, &dsl)
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

			metrics = append(metrics, vMetric{name: "datastore_capacity_size", help: "Datastore Total Size", value: float64(ds_capacity), labels: map[string]string{"datastore": ds_name}})
			metrics = append(metrics, vMetric{name: "datastore_capacity_free", help: "Datastore Size Free", value: float64(ds_freespace), labels: map[string]string{"datastore": ds_name}})
			metrics = append(metrics, vMetric{name: "datastore_capacity_used", help: "Datastore Size Used", value: float64(ds_used), labels: map[string]string{"datastore": ds_name}})
			metrics = append(metrics, vMetric{name: "datastore_capacity_uncommitted", help: "Datastore Size Uncommitted", value: float64(ds_uncommitted), labels: map[string]string{"datastore": ds_name}})
			metrics = append(metrics, vMetric{name: "datastore_capacity_pused", help: "Datastore Size", value: ds_pused, labels: map[string]string{"datastore": ds_name}})

		}

	}

	return metrics
}

func ClusterMetrics() []vMetric {
	ctx := context.Background()

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
			cls, _ := ClusterFromID(c, pool.Parent.Value)
			cname := cls.Name()

			// Get Quickstats form Resource Pool
			qs := pool.Summary.GetResourcePoolSummary().QuickStats
			memLimit := pool.Config.MemoryAllocation.Limit

			// Memory
			metrics = append(metrics, vMetric{name: "cluster_mem_ballooned", help: "Cluster Memory Ballooned", value: float64(qs.BalloonedMemory * 1024 * 1024), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "cluster_mem_compressed", help: "Cluster Memory ", value: float64(qs.CompressedMemory * 1024), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "cluster_mem_consumedOverhead", help: "Cluster Memory ", value: float64(qs.ConsumedOverheadMemory * 1024 * 1024), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "cluster_mem_distributedMemoryEntitlement", help: "Cluster Memory ", value: float64(qs.DistributedMemoryEntitlement * 1024 * 1024), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "cluster_mem_guest", help: "Cluster Memory ", value: float64(qs.GuestMemoryUsage * 1024 * 1024), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "cluster_mem_private", help: "Cluster Memory ", value: float64(qs.PrivateMemory * 1024 * 1024), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "cluster_mem_staticMemoryEntitlement", help: "Cluster Memory ", value: float64(qs.StaticMemoryEntitlement), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "cluster_mem_shared", help: "Cluster Memory ", value: float64(qs.SharedMemory * 1024 * 1024), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "cluster_mem_swapped", help: "Cluster Memory ", value: float64(qs.SwappedMemory * 1024 * 1024), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "cluster_mem_limit", help: "Cluster Memory ", value: float64(*memLimit * 1024 * 1024), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "cluster_mem_usage", help: "Cluster Memory ", value: float64(qs.HostMemoryUsage * 1024 * 1024), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "cluster_mem_overhead", help: "Cluster Memory ", value: float64(qs.OverheadMemory * 1024 * 1024), labels: map[string]string{"cluster": cname}})

			// CPU
			metrics = append(metrics, vMetric{name: "cluster_cpu_distributedCpuEntitlement", help: "Cluster CPU ", value: float64(qs.DistributedCpuEntitlement), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "cluster_cpu_demand", help: "Cluster CPU ", value: float64(qs.OverallCpuDemand), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "cluster_cpu_usage", help: "Cluster CPU ", value: float64(qs.OverallCpuUsage), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "cluster_cpu_staticCpuEntitlement", help: "Cluster CPU ", value: float64(qs.StaticCpuEntitlement), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "cluster_cpu_limit", help: "Cluster CPU ", value: float64(*pool.Config.CpuAllocation.Limit), labels: map[string]string{"cluster": cname}})
		}
	}

	for _, cl := range clusters {
		if cl.Summary != nil {
			cname := cl.Name
			qs := cl.Summary.GetComputeResourceSummary()

			// Memory
			metrics = append(metrics, vMetric{name: "cluster_mem_effective", help: "Cluster Mem ", value: float64(qs.EffectiveMemory * 1024 * 1024), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "cluster_mem_total", help: "CTotal Amount of Memory in Cluster", value: float64(qs.TotalMemory * 1024 * 1024), labels: map[string]string{"cluster": cname}})

			// CPU
			metrics = append(metrics, vMetric{name: "cluster_cpu_effective", help: "Cluster Mem ", value: float64(qs.EffectiveCpu), labels: map[string]string{"cluster": cname}})
			metrics = append(metrics, vMetric{name: "cluster_cpu_total", help: "Total Amount of CPU MHz in Cluster", value: float64(qs.TotalCpu), labels: map[string]string{"cluster": cname}})

			// Misc
			metrics = append(metrics, vMetric{name: "cluster_numHosts", help: "Number of Hypervisors in cluster ", value: float64(qs.NumHosts), labels: map[string]string{"cluster": cname}})

			// TODO fix missing ClusterComputeResourceSummary no only BaseComputeResourceSummary available
			//metrics = append(metrics, vMetric{ name: "cluster_numVmotions", help: "Cluster Number of vmotions ", value: float64(qs.NumHosts), labels: map[string]string{"cluster": cname}})

		}

	}

	return metrics
}

func HostMetrics() []vMetric {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
	err = v.Retrieve(ctx, []string{"HostSystem"}, []string{"summary"}, &hosts)
	if err != nil {
		log.Error(err.Error())
	}

	var metrics []vMetric

	for _, hs := range hosts {
		name := hs.Summary.Config.Name
		totalCPU := int64(hs.Summary.Hardware.CpuMhz) * int64(hs.Summary.Hardware.NumCpuCores)
		freeCPU := int64(totalCPU) - int64(hs.Summary.QuickStats.OverallCpuUsage)
		freeMemory := int64(hs.Summary.Hardware.MemorySize) - (int64(hs.Summary.QuickStats.OverallMemoryUsage) * 1024 * 1024)
		usedMemory := int64(hs.Summary.QuickStats.OverallMemoryUsage) * 1024 * 1024
		cpuPusage := math.Round((float64(hs.Summary.QuickStats.OverallCpuUsage) / float64(totalCPU)) * 100)
		memPusage := math.Round((float64(hs.Summary.QuickStats.OverallCpuUsage) / float64(totalCPU)) * 100)

		metrics = append(metrics, vMetric{name: "host_cpu_usage", help: "Hypervisors CPU usage", value: float64(hs.Summary.QuickStats.OverallCpuUsage), labels: map[string]string{"host": name}})
		metrics = append(metrics, vMetric{name: "host_cpu_total", help: "Hypervisors CPU Total", value: float64(totalCPU), labels: map[string]string{"host": name}})
		metrics = append(metrics, vMetric{name: "host_cpu_free", help: "Hypervisors CPU Free", value: float64(freeCPU), labels: map[string]string{"host": name}})
		metrics = append(metrics, vMetric{name: "host_cpu_pusage", help: "Hypervisors CPU Percent Usage", value: float64(cpuPusage), labels: map[string]string{"host": name}})

		metrics = append(metrics, vMetric{name: "host_mem_usage", help: "Hypervisors Memory Usage", value: float64(usedMemory), labels: map[string]string{"host": name}})
		metrics = append(metrics, vMetric{name: "host_mem_total", help: "Hypervisors Memory Total", value: float64(hs.Summary.Hardware.MemorySize), labels: map[string]string{"host": name}})
		metrics = append(metrics, vMetric{name: "host_mem_free", help: "Hypervisors Memory Free", value: float64(freeMemory), labels: map[string]string{"host": name}})
		metrics = append(metrics, vMetric{name: "host_mem_pusage", help: "Hypervisors Memory Percent Usage", value: float64(memPusage), labels: map[string]string{"host": name}})
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
