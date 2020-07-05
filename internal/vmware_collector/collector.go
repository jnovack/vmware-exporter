package vmwarecollector

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	secrets "github.com/ijustfool/docker-secrets"
	build "github.com/jnovack/go-version"
	"github.com/namsral/flag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// Collector TODO Comment
type Collector struct {
	desc string
}

var (
	defaultTimeout = 10 * time.Second
	hostname       = flag.String("hostname", "127.0.0.1", "hostname where ESXi or vCenter server is running")
	username       = flag.String("username", "root", "username for authentication (required)")
	password       = flag.String("password", "hunter2", "password for authentication (required)")
	passwordFile   = flag.String("password_file", "", "full path to the 'password' file (e.g. /run/secrets/password)")
	vcsim          = flag.Bool("vcsim", false, "Test environment")
	vmDebug        = flag.Bool("vmdebug", true, "debug vmware collector")
	vmStats        = flag.Bool("vmstats", true, "collect statistics from VMs")
)

func init() {
	log.Logger = log.With().Caller().Logger()
}

func timeTrack(ch chan<- prometheus.Metric, start time.Time, name string) {
	elapsed := time.Since(start)
	log.Debug().Msgf("%s took %.3fs", name, float64(elapsed.Milliseconds())/1000)

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
		prometheus.NewDesc("vmware_exporter", "github.com/jnovack/vmware_exporter", []string{}, prometheus.Labels{"version": build.Version}),
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
	if *vmStats == true {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer timeTrack(ch, time.Now(), "VirtualMachineMetrics")
			cm := VirtualMachineMetrics()
			for _, m := range cm {
				ch <- prometheus.MustNewConstMetric(
					prometheus.NewDesc(m.name, m.help, []string{}, m.labels),
					prometheus.GaugeValue,
					float64(m.value),
				)
			}

		}()
	}

	// Host Metrics
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer timeTrack(ch, time.Now(), "HostMetrics")
		cm := HostMetrics()
		for _, m := range cm {
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(m.name, m.help, []string{}, m.labels),
				prometheus.GaugeValue,
				float64(m.value),
			)
		}

	}()

	// Host Counters
	if *vmStats == true {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer timeTrack(ch, time.Now(), "HostCounters")
			cm := HostCounters()
			for _, m := range cm {
				ch <- prometheus.MustNewConstMetric(
					prometheus.NewDesc(m.name, m.help, []string{}, m.labels),
					prometheus.GaugeValue,
					float64(m.value),
				)
			}

		}()
	}

	// vcsim does not have any HBAs, and the query causes a panic.
	if *vcsim != true {
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
	}

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

	if *passwordFile != "" {
		dockerSecrets, err := secrets.NewDockerSecrets(filepath.Dir(*passwordFile))
		if err != nil {
			log.Fatal().Err(err).Msg("Could not open 'password_file'")
		}
		secret, err := dockerSecrets.Get("password")
		if err != nil {
			log.Fatal().Err(err).Msg("Could not retrieve password")
		}
		*password = secret
	}

	u, err := url.Parse("https://" + *hostname + vim25.Path)
	if err != nil {
		log.Fatal().Err(err).Msg("A fatal error occurred parsing the host")
	}

	u.User = url.UserPassword(*username, *password)
	log.Debug().Msg("Connecting to " + u.String())

	return govmomi.NewClient(ctx, u, true)
}

// GetClusters TODO Comment
func GetClusters(ctx context.Context, c *govmomi.Client, lst *[]mo.ClusterComputeResource) error {
	m := view.NewManager(c.Client)

	view, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"ClusterComputeResource"}, true)
	if err != nil {
		log.Error().Err(err).Msg("An error occurred.")
		return err
	}

	defer view.Destroy(ctx)

	err = view.Retrieve(ctx, []string{"ClusterComputeResource"}, []string{"name", "summary"}, lst)
	if err != nil {
		log.Error().Err(err).Msg("An error occurred.")
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
func GetVMLineage(ctx context.Context, client *govmomi.Client, host types.ManagedObjectReference) (mo.ManagedEntity, mo.ManagedEntity, mo.ManagedEntity, error) {
	var emptyEntity mo.ManagedEntity
	var hostEntity mo.ManagedEntity
	err := client.RetrieveOne(ctx, host.Reference(), []string{"name", "parent"}, &hostEntity)
	if err != nil {
		log.Error().Err(err).Msg("Unable to retrieve hostEntity.")
		return emptyEntity, emptyEntity, emptyEntity, err
	}

	var parent mo.ManagedEntity
	var datacenter mo.ManagedEntity
	var cluster mo.ManagedEntity
	parent = hostEntity
	for {
		if parent.Parent == nil {
			log.Trace().Str("name", parent.Name).Str("type", parent.Self.Type).Str("value", parent.Self.Value).Msg("root found")
			break
		}
		parent, err = getParent(ctx, client, parent.Parent.Reference())
		if err != nil {
			break
		}
		if parent.Self.Type == "ClusterComputeResource" {
			cluster = parent
			continue
		}
		if parent.Self.Type == "Datacenter" {
			datacenter = parent
			break
		}
	}

	return hostEntity, cluster, datacenter, err
}

// getParent will return the ManagedEntity object for the parent object of the current ManagedObjectReference
func getParent(ctx context.Context, client *govmomi.Client, objMOR types.ManagedObjectReference) (mo.ManagedEntity, error) {
	var err error
	var emptyEntity mo.ManagedEntity
	var thisEntity mo.ManagedEntity
	err = client.RetrieveOne(ctx, objMOR.Reference(), []string{"name", "parent"}, &thisEntity)
	if err != nil {
		log.Error().Err(err).Str("name", thisEntity.Name).Str("type", thisEntity.Self.Type).Str("value", thisEntity.Self.Value).Msg("Unable to retrieve thisEntity.")
		return emptyEntity, errors.New("unable to retrieve thisEntity")
	}
	log.Trace().Str("name", thisEntity.Name).Str("type", thisEntity.Self.Type).Str("value", thisEntity.Self.Value).Msg("Found thisEntity")

	var parentEntity mo.ManagedEntity
	err = client.RetrieveOne(ctx, thisEntity.Parent.Reference(), []string{"name", "parent"}, &parentEntity)
	if err != nil {
		log.Error().Err(err).Str("name", thisEntity.Name).Str("type", thisEntity.Self.Type).Str("value", thisEntity.Self.Value).Msg("Unable to retrieve parent of thisEntity.")
		return emptyEntity, errors.New("unable to retrieve parent of thisEntity")
	}

	log.Trace().Str("name", parentEntity.Name).Str("type", parentEntity.Self.Type).Str("value", parentEntity.Self.Value).Msg("Returning parentEntity")
	return parentEntity, nil
}

// GetMetricMap TODO Comment
func GetMetricMap(ctx context.Context, client *govmomi.Client) (MetricMap map[string]int32) {

	var pM mo.PerformanceManager
	err := client.RetrieveOne(ctx, *client.ServiceContent.PerfManager, nil, &pM)
	if err != nil {
		log.Fatal().Err(err).Msg("A fatal error occurred.")
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
		log.Fatal().Err(err).Msg("A fatal error occurred.")
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
		log.Fatal().Err(err).Msg("A fatal error occurred.")
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
