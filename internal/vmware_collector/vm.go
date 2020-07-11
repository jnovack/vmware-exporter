package vmwarecollector

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
)

// VirtualMachineMetrics TODO Comment
func VirtualMachineMetrics() []VMetric {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := NewClient(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("A fatal error occurred.")
	}

	defer c.Logout(ctx)

	m := view.NewManager(c.Client)

	view, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
	if err != nil {
		log.Error().Err(err).Msg("An error occurred.")
	}

	defer view.Destroy(ctx)

	var vms []mo.VirtualMachine

	// https://pubs.vmware.com/vi3/sdk/ReferenceGuide/vim.VirtualMachine.html#field_detail
	err = view.Retrieve(ctx, []string{"VirtualMachine"}, []string{"summary", "config", "name", "runtime", "resourcePool", "guestHeartbeatStatus"}, &vms)
	if err != nil {
		log.Error().Err(err).Msg("An error occurred.")
	}

	metricMap := GetMetricMap(ctx, c)

	idToName := make(map[int32]string)
	for k, v := range metricMap {
		idToName[v] = k
	}

	var metrics []VMetric

	for _, vm := range vms {
		// Labels - host, datacenter
		host, cluster, datacenter, err := GetVMLineage(ctx, c, vm.Runtime.Host.Reference())
		if err != nil {
			log.Error().Err(err).Str("vm", vm.Name).Str("host", vm.Runtime.Host.Value).Msg("Unable to get the VM lineage.")
			return nil
		}

		var rp *object.ResourcePool
		if vm.ResourcePool != nil {
			rp, err = getResourcePoolFromRef(c, vm.ResourcePool.Reference())
			if err != nil {
				log.Error().Err(err).Str("vm", vm.Name).Str("host", vm.Runtime.Host.Value).Msg("Unable to get the VM resource pool.")
			}
		}

		var clusterName string
		if cluster != nil {
			clusterName = cluster.Name()
		}

		// Calculations
		freeMemory := (int64(vm.Summary.Config.MemorySizeMB)) - (int64(vm.Summary.QuickStats.GuestMemoryUsage))

		status := -2
		switch string(vm.GuestHeartbeatStatus) {
		case "green":
			status = 1
		case "yellow":
			status = 0
		case "red":
			status = -1
		}

		powerState := 0

		switch string(vm.Runtime.PowerState) {
		case "poweredOn":
			powerState = 1
		case "suspended":
			powerState = -1
		}

		// Add Metrics
		metrics = append(metrics, VMetric{name: "vmware_vm_mem_total", help: "Memory size of the virtual machine, in MB.", value: float64(vm.Config.Hardware.MemoryMB), labels: map[string]string{"vm": vm.Name, "cluster": clusterName, "host": host.Name, "datacenter": datacenter.Name, "uuid": vm.Config.Uuid, "resource_pool": rp.Name()}})
		metrics = append(metrics, VMetric{name: "vmware_vm_mem_free", help: "Guest memory free statistics, in MB. This is also known as free guest memory. The number can be between 0 and the configured memory size of the virtual machine. Valid while the virtual machine is running.", value: float64(freeMemory), labels: map[string]string{"vm": vm.Name, "cluster": clusterName, "host": host.Name, "datacenter": datacenter.Name, "uuid": vm.Config.Uuid, "resource_pool": rp.Name()}})
		metrics = append(metrics, VMetric{name: "vmware_vm_mem_usage", help: "Guest memory utilization statistics, in MB. This is also known as active guest memory. The number can be between 0 and the configured memory size of the virtual machine. Valid while the virtual machine is running.", value: float64(vm.Summary.QuickStats.GuestMemoryUsage), labels: map[string]string{"vm": vm.Name, "cluster": clusterName, "host": host.Name, "datacenter": datacenter.Name, "uuid": vm.Config.Uuid, "resource_pool": rp.Name()}})

		metrics = append(metrics, VMetric{name: "vmware_vm_cpu_usage", help: "Basic CPU performance statistics, in MHz. Valid while the virtual machine is running.", value: float64(vm.Summary.QuickStats.OverallCpuUsage), labels: map[string]string{"vm": vm.Name, "cluster": clusterName, "host": host.Name, "datacenter": datacenter.Name, "uuid": vm.Config.Uuid, "resource_pool": rp.Name()}})
		metrics = append(metrics, VMetric{name: "vmware_vm_cpu_count", help: "Number of processors in the virtual machine.", value: float64(vm.Summary.Config.NumCpu), labels: map[string]string{"vm": vm.Name, "cluster": clusterName, "host": host.Name, "datacenter": datacenter.Name, "uuid": vm.Config.Uuid, "resource_pool": rp.Name()}})

		metrics = append(metrics, VMetric{name: "vmware_vm_heartbeat", help: "Overall alarm status on this node from VMware Tools.", value: float64(status), labels: map[string]string{"vm": vm.Name, "cluster": clusterName, "host": host.Name, "datacenter": datacenter.Name, "uuid": vm.Config.Uuid, "resource_pool": rp.Name()}})
		metrics = append(metrics, VMetric{name: "vmware_vm_powerstate", help: "The current power state of the virtual machine.", value: float64(powerState), labels: map[string]string{"vm": vm.Name, "cluster": clusterName, "host": host.Name, "datacenter": datacenter.Name, "uuid": vm.Config.Uuid, "resource_pool": rp.Name()}})

	}

	return metrics
}
