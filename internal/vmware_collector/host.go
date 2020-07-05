package vmwarecollector

import (
	"context"
	"math"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
)

// HostCounters Collects Hypervisor counters per Host
func HostCounters() []VMetric {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	c, err := NewClient(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("A fatal error occurred.")
	}

	defer c.Logout(ctx)

	m := view.NewManager(c.Client)

	view, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"HostSystem"}, true)
	if err != nil {
		log.Error().Err(err).Msg("An error occurred.")
	}

	defer view.Destroy(ctx)

	var hosts []mo.HostSystem
	err = view.Retrieve(ctx, []string{"HostSystem"}, []string{"name", "parent", "summary"}, &hosts)
	if err != nil {
		log.Error().Err(err).Msg("An error occurred.")
	}

	var metrics []VMetric

	for _, hs := range hosts {

		var cname string
		var name string
		// Get name of cluster the host is part of
		cls, err := ClusterFromRef(c, hs.Parent.Reference())
		if err != nil {
			log.Error().Err(err).Msg("connected locally to a EXSi host, not a vSphere")
			// return nil
			cname = hs.Name
			name = hs.Name
		} else {
			cname = cls.Name()
			cname = strings.ToLower(cname)
			name = hs.Summary.Config.Name
		}

		vmView, err := m.CreateContainerView(ctx, hs.Reference(), []string{"VirtualMachine"}, true)
		if err != nil {
			log.Error().Err(err).Str("hs.name", hs.Name).Msg("An error occurred.")
		}

		var vms []mo.VirtualMachine

		err2 := vmView.RetrieveWithFilter(ctx, []string{"VirtualMachine"}, []string{"name", "runtime"}, &vms, property.Filter{"runtime.powerState": "poweredOn"})
		if err2 != nil {
			//	log.Error().Err(err2.Error() +": HostCounters - poweron").Msg("An error occurred.")
		}

		poweredOn := len(vms)

		err = vmView.Retrieve(ctx, []string{"VirtualMachine"}, []string{"name", "summary.config", "runtime.powerState"}, &vms)
		if err != nil {
			log.Error().Err(err).Msg("An error occurred retrieving VMs.")
		}

		total := len(vms)

		metrics = append(metrics, VMetric{name: "vmware_host_vm_on", help: "Number of powered on virtual machines running on host", value: float64(poweredOn), labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, VMetric{name: "vmware_host_vm_total", help: "Total number of virtual machines registered on host", value: float64(total), labels: map[string]string{"host": name, "cluster": cname}})

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
			vMem = vMem + int64(vm.Summary.Config.MemorySizeMB)

			pwr := string(vm.Runtime.PowerState)
			//fmt.Println(pwr)
			if pwr == "poweredOn" {
				vCPUOn = vCPUOn + int64(vm.Summary.Config.NumCpu)
				vMemOn = vMemOn + int64(vm.Summary.Config.MemorySizeMB)
			}
		}

		metrics = append(metrics, VMetric{name: "vmware_host_cpu_all", help: "Total number of virtual CPUs configured on host", value: float64(vCPU), labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, VMetric{name: "vmware_host_mem_all", help: "Total amount of virtual memory (in MB) configured on host", value: float64(vMem), labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, VMetric{name: "vmware_host_cpu_used", help: "Number of virtual CPUs used by VMs on host", value: float64(vCPUOn), labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, VMetric{name: "vmware_host_mem_used", help: "Memory used (in MB) by VMs on host", value: float64(vMemOn), labels: map[string]string{"host": name, "cluster": cname}})

		cores := hs.Summary.Hardware.NumCpuCores
		metrics = append(metrics, VMetric{name: "vmware_host_cores", help: "Number of physical cores available on host", value: float64(cores), labels: map[string]string{"host": name, "cluster": cname}})

		vmView.Destroy(ctx)
	}

	return metrics
}

// HostMetrics Collects Hypervisor metrics
func HostMetrics() []VMetric {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	c, err := NewClient(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("A fatal error occurred.")
	}

	defer c.Logout(ctx)

	m := view.NewManager(c.Client)

	view, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"HostSystem"}, true)
	if err != nil {
		log.Error().Err(err).Msg("An error occurred.")
	}

	defer view.Destroy(ctx)

	var hosts []mo.HostSystem
	err = view.Retrieve(ctx, []string{"HostSystem"}, []string{"summary", "parent", "vm"}, &hosts)
	if err != nil {
		log.Error().Err(err).Msg("An error occurred.")
	}

	var metrics []VMetric

	for _, hs := range hosts {
		var cname string

		// Get name of cluster the host is part of
		cls, err := ClusterFromRef(c, hs.Parent.Reference())
		if err != nil {
			log.Error().Err(err).Msg("An error occurred.")
			cname = hs.Name
			// return nil
		} else {
			cname = cls.Name()
			cname = strings.ToLower(cname)
		}

		name := hs.Summary.Config.Name
		totalCPU := int64(hs.Summary.Hardware.CpuMhz) * int64(hs.Summary.Hardware.NumCpuCores)
		freeCPU := int64(totalCPU) - int64(hs.Summary.QuickStats.OverallCpuUsage)
		cpuPusage := math.Round((float64(hs.Summary.QuickStats.OverallCpuUsage) / float64(totalCPU)) * 100)

		totalMemory := float64(hs.Summary.Hardware.MemorySize / 1024 / 1024 / 1024)
		usedMemory := float64(hs.Summary.QuickStats.OverallMemoryUsage / 1024)
		freeMemory := totalMemory - usedMemory
		memPusage := math.Round((usedMemory / totalMemory) * 100)

		metrics = append(metrics, VMetric{name: "vmware_host_cpu_usage", help: "Hypervisors CPU usage", value: float64(hs.Summary.QuickStats.OverallCpuUsage), labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, VMetric{name: "vmware_host_cpu_total", help: "Hypervisors CPU Total", value: float64(totalCPU), labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, VMetric{name: "vmware_host_cpu_free", help: "Hypervisors CPU Free", value: float64(freeCPU), labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, VMetric{name: "vmware_host_cpu_usage_percent", help: "Hypervisors CPU Percent Usage", value: float64(cpuPusage), labels: map[string]string{"host": name, "cluster": cname}})

		metrics = append(metrics, VMetric{name: "vmware_host_mem_usage", help: "Hypervisors Memory Usage", value: usedMemory, labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, VMetric{name: "vmware_host_mem_total", help: "Hypervisors Memory Total", value: totalMemory, labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, VMetric{name: "vmware_host_mem_free", help: "Hypervisors Memory Free", value: float64(freeMemory), labels: map[string]string{"host": name, "cluster": cname}})
		metrics = append(metrics, VMetric{name: "vmware_host_mem_usage_percent", help: "Hypervisors Memory Percent Usage", value: float64(memPusage), labels: map[string]string{"host": name, "cluster": cname}})

	}

	return metrics
}

// HostHBAStatus Report status of the HBA attached to a hypervisor to be able to monitor if a hba goes offline
func HostHBAStatus() []VMetric {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	c, err := NewClient(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to connect to server.")
	}

	defer c.Logout(ctx)

	m := view.NewManager(c.Client)

	view, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"HostSystem"}, true)
	if err != nil {
		log.Error().Err(err).Msg("An error occurred.")
	}

	defer view.Destroy(ctx)

	var hosts []mo.HostSystem
	err = view.Retrieve(ctx, []string{"HostSystem"}, []string{"name", "parent"}, &hosts)
	if err != nil {
		log.Error().Err(err).Msg("An error occurred.")
	}

	var metrics []VMetric

	for _, host := range hosts {
		var cname string
		// Get name of cluster the host is part of
		cls, err := ClusterFromRef(c, host.Parent.Reference())
		if err != nil {
			log.Debug().Str("msg", err.Error()).Msgf("%s is connected locally to a EXSi host, not a vSphere cluster", *hostname)
			cname = *hostname
		} else {
			cname = strings.ToLower(cls.Name())
		}

		hcm := object.NewHostConfigManager(c.Client, host.Reference())
		ss, err := hcm.StorageSystem(ctx)
		if err != nil {
			log.Error().Err(err).Msg("An error occurred.")
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
				metrics = append(metrics, VMetric{name: "vmware_host_hba_status", help: "Hypervisors hba Online status, 1 == Online", value: float64(status), labels: map[string]string{"host": host.Name, "cluster": cname, "hba": hba.Device}})
			}

		}
	}

	return metrics
}
