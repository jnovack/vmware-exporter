package main

import (
	"context"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/magiconair/properties"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/performance"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"net/url"
	"os"
	"strings"
	"time"
)

type TestConfiguration struct {
	Host     string
	User     string
	Password string
	Debug    bool
}

var testcfg TestConfiguration

func main() {
	//TestPerf()
	ClusterTestPerf()

}

func init() {
	// Get config details
	if os.Getenv("HOST") != "" && os.Getenv("USERID") != "" && os.Getenv("PASSWORD") != "" {
		if os.Getenv("DEBUG") == "True" {
			testcfg = TestConfiguration{Host: os.Getenv("HOST"), User: os.Getenv("USERID"), Password: os.Getenv("PASSWORD"), Debug: true}
		} else {
			testcfg = TestConfiguration{Host: os.Getenv("HOST"), User: os.Getenv("USERID"), Password: os.Getenv("PASSWORD"), Debug: false}
		}

	} else {
		p := properties.MustLoadFiles([]string{
			"../config.properties",
		}, properties.UTF8, true)

		testcfg = TestConfiguration{Host: p.MustGetString("host"), User: p.MustGetString("user"), Password: p.MustGetString("password"), Debug: p.MustGetBool("debug")}
	}

}

// Connect to vCenter
func NewTestClient(ctx context.Context) (*govmomi.Client, error) {

	u, err := url.Parse("https://" + testcfg.Host + vim25.Path)
	if err != nil {
		log.Fatal(err)
	}
	u.User = url.UserPassword(testcfg.User, testcfg.Password)
	log.Debug("Connecting to " + u.String())

	return govmomi.NewClient(ctx, u, true)
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


func ClusterTestPerf() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c, err := NewTestClient(ctx)
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
	mlist,err := pm.CounterInfoByKey(ctx)
	if err != nil {
		log.Error(err.Error())

	}

	for _, cls := range lst {


		fmt.Println(cls.Name)
		am,_ := pm.AvailableMetric(ctx,cls.Reference(),300)

		var pqList []types.PerfMetricId
		for _,v := range am {

			if strings.Contains(mlist[v.CounterId].Name(),"vmop"){
				pqList = append(pqList,v)
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
				name := strings.TrimLeft(mlist[series.Id.CounterId].Name(),"vmop.")
				name = strings.TrimRight(name,".latest")
				fmt.Print(name + ": ")
				//fmt.Print(mlist[series.Id.CounterId].Name() + ": ")
				fmt.Println(series.Value[0])
			}
		}

	}

}

func TestPerf() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c, err := NewTestClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	defer c.Logout(ctx)
	/*
		perfMan := performance.NewManager(c.Client)

		infoByName, _ := perfMan.CounterInfoByName(ctx)

		for k,_ := range infoByName{
			fmt.Println(k)

		}
	*/

	// Create view of VirtualMachine objects
	m := view.NewManager(c.Client)

	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
	if err != nil {
		log.Fatal(err)
	}

	defer v.Destroy(ctx)

	var vms []mo.VirtualMachine
	err = v.RetrieveWithFilter(ctx, []string{"VirtualMachine"}, []string{"summary"}, &vms, property.Filter{"name": "se1t5xitest002.gameop.net"})
	if err != nil {
		log.Fatal(err)
	}

	PerfQuery(ctx, c, []string{"cpu.ready.summation", "cpu.usage.none"}, vms[0].GetManagedEntity())

	/*

		out,err := perfMan.AvailableMetric(ctx,vms[0].Reference(),20)
		if err != nil {
			log.Fatal(err)
		}
		for k,v := range out {
			fmt.Println(k,v.CounterId)
		}
	*/

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

func PerfQuery(ctx context.Context, c *govmomi.Client, metrics []string, entity mo.ManagedEntity) map[string]int64 {

	var pM mo.PerformanceManager
	err := c.RetrieveOne(ctx, *c.ServiceContent.PerfManager, nil, &pM)
	if err != nil {
		log.Fatal(err)
	}

	metricMap := GetMetricMap(ctx, c)

	idToName := make(map[int32]string)
	for k, v := range metricMap {
		idToName[v] = k
	}

	var pmidList []types.PerfMetricId
	for _, v := range metrics {
		mid := types.PerfMetricId{CounterId: metricMap[v]}
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
			fmt.Print(idToName[series.Id.CounterId] + ": ")
			var sum int64
			for _, v := range series.Value {
				sum = sum + v
			}
			fmt.Println(sum / 3)
		}
	}
	return data
}


