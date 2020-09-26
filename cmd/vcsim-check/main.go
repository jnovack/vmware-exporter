package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"sync"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"golang.org/x/net/context"
)

var vsphereURL = "https://name:pass@localhost:8989/sdk"
var insecureFlag = true

func exit(err error) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	os.Exit(1)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	flag.Parse()
	u, err := url.Parse(vsphereURL)
	if err != nil {
		exit(err)
	}
	c, err := govmomi.NewClient(ctx, u, insecureFlag)
	if err != nil {
		exit(err)
	}
	f := find.NewFinder(c.Client, true)
	dcs := []string{"DC0"}
	var group sync.WaitGroup
	for _, dc := range dcs {
		group.Add(1)
		go func(ctx context.Context, dc string, f *find.Finder, c *govmomi.Client, group *sync.WaitGroup) {
			defer group.Done()
			dcObj, err := f.DatacenterOrDefault(ctx, dc)
			if err != nil {
				exit(err)
			}
			f.SetDatacenter(dcObj)
			vms, err := f.VirtualMachineList(ctx, "*")
			if err != nil {
				exit(err)
			}
			pc := property.DefaultCollector(c.Client)
			var refs []types.ManagedObjectReference
			for _, vm := range vms {
				refs = append(refs, vm.Reference())
			}

			var vmt []mo.VirtualMachine
			err = pc.Retrieve(ctx, refs, []string{"name", "resourcePool"}, &vmt)
			if err != nil {
				exit(err)
			}
			for _, vm := range vmt {
				fmt.Println(vm.Name, vm.ResourcePool)
			}

			var rps []*object.ResourcePool
			rps, err = f.ResourcePoolList(ctx, "*")
			if err != nil {
				exit(err)
			}
			for _, rp := range rps {
				fmt.Println(rp)
			}

		}(ctx, dc, f, c, &group)
	}
	group.Wait()
	fmt.Println("Done.")
}
