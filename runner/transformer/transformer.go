package transformer

import (
	"context"
	"fmt"
	"plugin"

	typesv1 "github.com/michaelhenkel/fabricmanager/api/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Transform transforms a device configuration
func Transform(device *typesv1.Device, client client.Client) {
	fmt.Printf("starting to transform device configuration %s\n", device.Name)
	/*
		rand.Seed(time.Now().UnixNano())
		n := rand.Intn(20) // n will be between 0 and 10
	*/
	pluginPath := "plugins/" + device.Spec.Vendor + ".so"
	p, err := plugin.Open(pluginPath)
	if err != nil {
		panic(err)
	}
	interfaceHandler, err := p.Lookup("InterfaceHandler")
	if err != nil {
		panic(err)
	}

	for idx, activeInterfaceRefStatus := range device.Status.Interfaces {
		var activeInterface typesv1.Interface
		if err := client.Get(context.Background(), types.NamespacedName{Name: activeInterfaceRefStatus.InterfaceRef.Name, Namespace: activeInterfaceRefStatus.InterfaceRef.Namespace}, &activeInterface); err != nil {
			fmt.Println(err)
		}
		err = interfaceHandler.(func(*typesv1.Interface) error)(&activeInterface)
		if err != nil {
			failed := typesv1.FAILED
			device.Status.Interfaces[idx].CommitStatus = &failed
		} else {
			success := typesv1.SUCCESS
			device.Status.Interfaces[idx].CommitStatus = &success
		}
	}
	for _, activeInterfaceRefStatus := range device.Status.Interfaces {
		fmt.Printf("Interface %s status %s\n", activeInterfaceRefStatus.InterfaceRef.Name, *activeInterfaceRefStatus.CommitStatus)
	}
	if err := client.Status().Update(context.Background(), device); err != nil {
		fmt.Println(err)
	}

}
