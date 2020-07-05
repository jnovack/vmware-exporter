package vmwarecollector

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
)

// DatacenterMetrics TODO Comment
func DatacenterMetrics(ch chan<- prometheus.Metric) []VMetric {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	c, err := NewClient(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("A fatal error occurred.")
	}

	defer c.Logout(ctx)

	m := view.NewManager(c.Client)

	view, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"Datacenter"}, true)
	if err != nil {
		log.Error().Err(err).Msg("An error occurred.")
	}

	defer view.Destroy(ctx)

	var metrics []VMetric
	var arrDC []mo.Datacenter

	err = view.Retrieve(ctx, []string{"Datacenter"}, []string{"name", "datastore", "network"}, &arrDC)
	if err != nil {
		log.Error().Err(err).Msg("An error occurred.")
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
