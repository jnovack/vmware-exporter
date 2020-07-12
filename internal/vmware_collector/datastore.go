package vmwarecollector

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
)

// DatastoreMetrics TODO Comment
func DatastoreMetrics(objDC mo.Datacenter) []VMetric {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	c, err := NewClient(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("A fatal error occurred.")
	}

	defer c.Logout(ctx)

	m := view.NewManager(c.Client)

	vmgr, err := m.CreateContainerView(ctx, objDC.Reference(), []string{"ClusterComputeResource"}, true)
	if err != nil {
		log.Error().Err(err).Msg("An error occurred.")
	}

	defer vmgr.Destroy(ctx)

	var metrics []VMetric

	var lst []mo.ClusterComputeResource
	err = vmgr.Retrieve(ctx, []string{"ClusterComputeResource"}, []string{"name", "datastore"}, &lst)
	if err != nil {
		log.Error().Err(err).Msg("An error occurred.")
	}
	for _, cls := range lst {

		cname := cls.Name

		var dsl []mo.Datastore
		pc := c.PropertyCollector()
		pc.Retrieve(ctx, cls.Datastore, []string{"summary", "name"}, &dsl)

		for _, ds := range dsl {
			if ds.Summary.Accessible {
				dsCapacity := ds.Summary.Capacity
				dsFreeSpace := ds.Summary.FreeSpace
				dsUsed := dsCapacity - dsFreeSpace

				metrics = append(metrics, VMetric{name: "vmware_datastore_size", help: "Maximum capacity of this datastore, in bytes.", value: float64(dsCapacity), labels: map[string]string{"datastore": ds.Summary.Name, "cluster": cname, "datacenter": objDC.Name}})
				metrics = append(metrics, VMetric{name: "vmware_datastore_free", help: "Available space of this datastore, in bytes.", value: float64(dsFreeSpace), labels: map[string]string{"datastore": ds.Summary.Name, "cluster": cname, "datacenter": objDC.Name}})
				metrics = append(metrics, VMetric{name: "vmware_datastore_used", help: "Used space of this datastore, in bytes.", value: float64(dsUsed), labels: map[string]string{"datastore": ds.Summary.Name, "cluster": cname, "datacenter": objDC.Name}})
			}

		}
	}

	return metrics
}
