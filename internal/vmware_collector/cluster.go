package vmwarecollector

import (
	"context"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
)

// ClusterMetrics TODO Comment
func ClusterMetrics(ch chan<- prometheus.Metric, objDC mo.Datacenter) []VMetric {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	c, err := NewClient(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("A fatal error occurred.")
	}

	defer c.Logout(ctx)

	var arrCLS []mo.ClusterComputeResource
	e2 := GetClusters(ctx, c, &arrCLS)
	if e2 != nil {
		log.Error().Err(e2).Msg("An error occurred.")
	}

	m := view.NewManager(c.Client)

	view, err := m.CreateContainerView(ctx, objDC.Reference(), []string{"ResourcePool"}, true)
	if err != nil {
		log.Error().Err(err).Msg("An error occurred.")
	}

	defer view.Destroy(ctx)

	var pools []mo.ResourcePool
	err = view.Retrieve(ctx, []string{"ResourcePool"}, []string{"summary", "name", "parent", "config"}, &pools)
	if err != nil {
		log.Error().Err(err).Msg("An error occurred.")
		//return err
	}

	var metrics []VMetric

	for _, pool := range pools {
		if pool.Config.Entity == nil {
			// Get Cluster name from Resource Pool Parent
			cluster, err := getCluster(c, pool.Reference())
			if err != nil {
				log.Debug().Err(err).Msgf("%s is connected locally to a EXSi host, not a vSphere cluster", *hostname)
				break
			}

			metrics = append(metrics, VMetric{name: "vmware_pool_mem_limit", help: "TODO ADD DESCRIPTION", value: float64(*pool.Config.MemoryAllocation.Limit), labels: map[string]string{"cluster": cluster.Name, "pool": pool.GetManagedEntity().Name}})
			metrics = append(metrics, VMetric{name: "vmware_pool_mem_reservation", help: "TODO ADD DESCRIPTION", value: float64(*pool.Config.MemoryAllocation.Reservation), labels: map[string]string{"cluster": cluster.Name, "pool": pool.GetManagedEntity().Name}})

			metrics = append(metrics, VMetric{name: "vmware_pool_cpu_limit", help: "TODO ADD DESCRIPTION", value: float64(*pool.Config.CpuAllocation.Limit), labels: map[string]string{"cluster": cluster.Name, "pool": pool.GetManagedEntity().Name}})
			metrics = append(metrics, VMetric{name: "vmware_pool_cpu_reservation", help: "TODO ADD DESCRIPTION", value: float64(*pool.Config.CpuAllocation.Reservation), labels: map[string]string{"cluster": cluster.Name, "pool": pool.GetManagedEntity().Name}})

			// // Get Quickstats form Resource Pool
			// qs := pool.Summary.GetResourcePoolSummary().QuickStats

			// if qs != nil {
			// 	// Memory
			// 	metrics = append(metrics, VMetric{name: "vmware_pool_mem_ballooned", help: "The size of the balloon driver in a virtual machine, in MB. ", value: float64(qs.BalloonedMemory), labels: map[string]string{"cluster": cluster.Name, "pool": pool.Name}})
			// 	metrics = append(metrics, VMetric{name: "vmware_pool_mem_compressed", help: "The amount of compressed memory currently consumed by VM, in KB", value: float64(qs.CompressedMemory), labels: map[string]string{"cluster": cluster.Name, "pool": pool.Name}})
			// 	metrics = append(metrics, VMetric{name: "vmware_pool_mem_consumedOverhead", help: "The amount of overhead memory, in MB, currently being consumed to run a VM.", value: float64(qs.ConsumedOverheadMemory), labels: map[string]string{"cluster": cluster.Name, "pool": pool.Name}})
			// 	metrics = append(metrics, VMetric{name: "vmware_pool_mem_distributedMemoryEntitlement", help: "This is the amount of CPU resource, in MHz, that this VM is entitled to, as calculated by DRS.", value: float64(qs.DistributedMemoryEntitlement), labels: map[string]string{"cluster": cluster.Name, "pool": pool.Name}})
			// 	metrics = append(metrics, VMetric{name: "vmware_pool_mem_guest", help: "Guest memory utilization statistics, in MB. This is also known as active guest memory.", value: float64(qs.GuestMemoryUsage), labels: map[string]string{"cluster": cluster.Name, "pool": pool.Name}})
			// 	metrics = append(metrics, VMetric{name: "vmware_pool_mem_private", help: "The portion of memory, in MB, that is granted to a virtual machine from non-shared host memory.", value: float64(qs.PrivateMemory), labels: map[string]string{"cluster": cluster.Name, "pool": pool.Name}})
			// 	metrics = append(metrics, VMetric{name: "vmware_pool_mem_staticMemoryEntitlement", help: "The static memory resource entitlement for a virtual machine, in MB.", value: float64(qs.StaticMemoryEntitlement), labels: map[string]string{"cluster": cluster.Name, "pool": pool.Name}})
			// 	metrics = append(metrics, VMetric{name: "vmware_pool_mem_shared", help: "The portion of memory, in MB, that is granted to a virtual machine from host memory that is shared between VMs.", value: float64(qs.SharedMemory), labels: map[string]string{"cluster": cluster.Name, "pool": pool.Name}})
			// 	metrics = append(metrics, VMetric{name: "vmware_pool_mem_swapped", help: "The portion of memory, in MB, that is granted to a virtual machine from the host's swap space.", value: float64(qs.SwappedMemory), labels: map[string]string{"cluster": cluster.Name, "pool": pool.Name}})
			// 	metrics = append(metrics, VMetric{name: "vmware_pool_mem_limit", help: "Cluster Memory, in MB", value: float64(*pool.Config.MemoryAllocation.Limit), labels: map[string]string{"cluster": cluster.Name, "pool": pool.Name}})
			// 	metrics = append(metrics, VMetric{name: "vmware_pool_mem_usage", help: "Host memory utilization statistics, in MB. This is also known as consumed host memory.", value: float64(qs.HostMemoryUsage), labels: map[string]string{"cluster": cluster.Name, "pool": pool.Name}})
			// 	metrics = append(metrics, VMetric{name: "vmware_pool_mem_overhead", help: "The amount of memory resource (in MB) that will be used by a virtual machine above its guest memory requirements.", value: float64(qs.OverheadMemory), labels: map[string]string{"cluster": cluster.Name, "pool": pool.Name}})

			// 	// CPU
			// 	metrics = append(metrics, VMetric{name: "vmware_pool_cpu_distributedCpuEntitlement", help: "This is the amount of CPU resource, in MHz, that this VM is entitled to.", value: float64(qs.DistributedCpuEntitlement), labels: map[string]string{"cluster": cluster.Name, "pool": pool.Name}})
			// 	metrics = append(metrics, VMetric{name: "vmware_pool_cpu_demand", help: "Basic CPU performance statistics, in MHz.", value: float64(qs.OverallCpuDemand), labels: map[string]string{"cluster": cluster.Name, "pool": pool.Name}})
			// 	metrics = append(metrics, VMetric{name: "vmware_pool_cpu_usage", help: "Basic CPU performance statistics, in MHz.", value: float64(qs.OverallCpuUsage), labels: map[string]string{"cluster": cluster.Name, "pool": pool.Name}})
			// 	metrics = append(metrics, VMetric{name: "vmware_pool_cpu_staticCpuEntitlement", help: "The static CPU resource entitlement for a virtual machine, in MHz.", value: float64(qs.StaticCpuEntitlement), labels: map[string]string{"cluster": cluster.Name, "pool": pool.Name}})
			// 	metrics = append(metrics, VMetric{name: "vmware_pool_cpu_limit", help: "Cluster CPU, MHz", value: float64(*pool.Config.CpuAllocation.Limit), labels: map[string]string{"cluster": cluster.Name, "pool": pool.Name}})
			// }
		}
	}

	waitGroupCLS := sync.WaitGroup{}

	for _, objCLS := range arrCLS {
		if objCLS.Summary != nil {
			qs := objCLS.Summary.GetComputeResourceSummary()

			// Memory
			metrics = append(metrics, VMetric{name: "vmware_cluster_mem_effective", help: "Effective memory resources available to run virtual machines, in MB.", value: float64(qs.EffectiveMemory), labels: map[string]string{"cluster": objCLS.Name}})
			metrics = append(metrics, VMetric{name: "vmware_cluster_mem_total", help: "Aggregated memory resources of all hosts, in MB.", value: float64(qs.TotalMemory / 1024 / 1024), labels: map[string]string{"cluster": objCLS.Name}})

			// CPU
			metrics = append(metrics, VMetric{name: "vmware_cluster_cpu_effective", help: "Effective CPU resources available to run virtual machines, in MHz.", value: float64(qs.EffectiveCpu), labels: map[string]string{"cluster": objCLS.Name}})
			metrics = append(metrics, VMetric{name: "vmware_cluster_cpu_total", help: "Aggregated CPU resources of all hosts, in MHz.", value: float64(qs.TotalCpu), labels: map[string]string{"cluster": objCLS.Name}})
			metrics = append(metrics, VMetric{name: "vmware_cluster_cpu_threads", help: "Aggregated number of CPU threads.", value: float64(qs.NumCpuThreads), labels: map[string]string{"cluster": objCLS.Name}})
			metrics = append(metrics, VMetric{name: "vmware_cluster_cpu_cores", help: "Number of physical CPU cores. Physical CPU cores are the processors contained by a CPU package.", value: float64(qs.NumCpuCores), labels: map[string]string{"cluster": objCLS.Name}})

			// Misc
			metrics = append(metrics, VMetric{name: "vmware_cluster_hosts_effective", help: "Total number of effective hosts.", value: float64(qs.NumEffectiveHosts), labels: map[string]string{"cluster": objCLS.Name}})
			metrics = append(metrics, VMetric{name: "vmware_cluster_hosts_total", help: "Total number of hosts.", value: float64(qs.NumHosts), labels: map[string]string{"cluster": objCLS.Name}})

		}
	}

	waitGroupCLS.Wait()

	return metrics
}

/*
// ClusterCounters TODO Comment
func ClusterCounters() []VMetric {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	c, err := NewClient(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("A fatal error occurred.")
	}

	defer c.Logout(ctx)

	m := view.NewManager(c.Client)

	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"ClusterComputeResource"}, true)
	if err != nil {
		log.Error().Err(err).Msg("An error occurred.")

	}

	defer v.Destroy(ctx)

	var lst []mo.ClusterComputeResource
	err = v.Retrieve(ctx, []string{"ClusterComputeResource"}, []string{"name"}, &lst)
	if err != nil {
		log.Error().Err(err).Msg("An error occurred.")

	}

	pm := performance.NewManager(c.Client)
	mlist, err := pm.CounterInfoByKey(ctx)
	if err != nil {
		log.Error().Err(err).Msg("An error occurred.")

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
			log.Fatal().Err(err).Msg("A fatal error occurred.")
		}

		// vmware_cluster_vmop_numChangeDS{cluster="ucs"} 1
		// vmware_cluster_vmop_numChangeHo{cluster="ucs"} 8
		// vmware_cluster_vmop_numChangeHostDS{cluster="ucs"} 0
		// vmware_cluster_vmop_numClon{cluster="ucs"} 0
		// vmware_cluster_vmop_numCr{cluster="ucs"} 3
		// vmware_cluster_vmop_numDeploy{cluster="ucs"} 0
		// vmware_cluster_vmop_numDestroy{cluster="ucs"} 4
		// vmware_cluster_vmop_numPoweroff{cluster="ucs"} 3
		// vmware_cluster_vmop_numPoweron{cluster="ucs"} 11
		// vmware_cluster_vmop_numR{cluster="ucs"} 1
		// vmware_cluster_vmop_numRebootGu{cluster="ucs"} 0
		// vmware_cluster_vmop_numReconfigur{cluster="ucs"} 100
		// vmware_cluster_vmop_numRegister{cluster="ucs"} 0
		// vmware_cluster_vmop_numSVMotion{cluster="ucs"} 3
		// vmware_cluster_vmop_numShutdownGu{cluster="ucs"} 1
		// vmware_cluster_vmop_numStandbyGu{cluster="ucs"} 0
		// vmware_cluster_vmop_numSuspend{cluster="ucs"} 0
		// vmware_cluster_vmop_numUnregister{cluster="ucs"} 0
		// vmware_cluster_vmop_numVMotion{cluster="ucs"} 10
		// vmware_cluster_vmop_numXVMotion{cluster="ucs"} 0

		for _, base := range response.Returnval {
			metric := base.(*types.PerfEntityMetric)
			for _, baseSeries := range metric.Value {
				series := baseSeries.(*types.PerfMetricIntSeries)
				name := strings.TrimLeft(mlist[series.Id.CounterId].Name(), "vmop.")
				name = strings.TrimRight(name, ".latest")
				metrics = append(metrics, VMetric{name: "vmware_cluster_vmop_" + name, help: "vmops counter ", value: float64(series.Value[0]), labels: map[string]string{"cluster": cname}})

			}
		}

	}
	return metrics
}
*/
