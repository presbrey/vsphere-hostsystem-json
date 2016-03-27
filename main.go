package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"golang.org/x/net/context"
)

var (
	csv  = flag.Bool("csv", false, "output csv instead of json")
	path = flag.String("path", "/", "vSphere Inventory Root")
	uri  = flag.String("uri", "https://user:password@vsphere/sdk", "vSphere URI (or $URI)")

	ctx = context.Background()
	db  = map[string]map[string][]string{}
)

func init() {
	flag.Parse()
	v := os.Getenv("URI")
	if v != "" {
		*uri = v
	}
}

func walk(ref object.Reference) error {
	switch elt := ref.(type) {

	case *object.Datacenter:
		folders, err := elt.Folders(ctx)
		if err != nil {
			return err
		}
		children, err := folders.VmFolder.Children(ctx)
		if err != nil {
			return err
		}
		for _, child := range children {
			walk(child)
		}

	case *object.Folder:
		children, err := elt.Children(ctx)
		if err != nil {
			return err
		}
		for _, child := range children {
			walk(child)
		}

	case *object.VirtualMachine:
		var mvm mo.VirtualMachine
		err := elt.Properties(ctx, elt.Reference(), []string{"guest"}, &mvm)
		if err != nil {
			return err
		}

		name, err := elt.Name(ctx)
		if err != nil {
			return err
		}

		netmap := map[string][]string{}
		for _, nic := range mvm.Guest.Net {
			if nic.IpConfig == nil {
				continue
			}
			if _, ex := netmap[nic.MacAddress]; !ex {
				netmap[nic.MacAddress] = []string{}
			}
			for _, ip := range nic.IpConfig.IpAddress {
				netmap[nic.MacAddress] = append(netmap[nic.MacAddress], ip.IpAddress)
				if *csv {
					fmt.Println(name + "\t" + nic.MacAddress + "\t" + ip.IpAddress)
				}
			}
		}

		if _, ex := db[name]; !ex {
			db[name] = map[string][]string{}
		}
		for k, v := range netmap {
			db[name][k] = v
		}

	default:
		log.Printf("skipping %+T\n", elt)

	}
	return nil
}

func stdout() error {
	uri, err := soap.ParseURL(*uri)
	if err != nil {
		return err
	}
	c, err := govmomi.NewClient(ctx, uri, true)
	if err != nil {
		return err
	}
	s := object.NewSearchIndex(c.Client)
	ref, err := s.FindByInventoryPath(ctx, *path)
	if err != nil {
		return err
	}
	err = walk(ref)
	if err != nil {
		return err
	}
	if *csv {
		return nil
	}
	return json.NewEncoder(os.Stdout).Encode(db)
}

func main() {
	err := stdout()
	if err != nil {
		log.Println(err)
	}
}
