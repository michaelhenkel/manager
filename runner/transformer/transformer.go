package transformer

import (
	"context"
	"fmt"
	"time"

	typesv1 "github.com/michaelhenkel/fabricmanager/api/v1"
	nbc "github.com/michaelhenkel/fabricmanager/nbc"
	devicePlugin "github.com/michaelhenkel/fabricmanager/runner/plugins"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Transform transforms a device configuration
func Transform(configChannel *nbc.NonBlockingChan, client client.Client) {
	for {
		v, ok := <-configChannel.Recv
		if ok {
			device := v.(*typesv1.Device)
			callDeviceConfigurator(device, client)
			fmt.Printf("%d items in queue\n", configChannel.Len())
		} else {
			fmt.Println("Channel closed")
		}
	}

}

func callDeviceConfigurator(device *typesv1.Device, client client.Client) {
	if device.Name == "" {
		fmt.Println("Device name empty, device deleted?")
	} else {
		fmt.Println("received message", device.Name)
		ticker := time.Tick(time.Second)
		for i := 1; i <= 10; i++ {
			<-ticker
			fmt.Printf("\x0cOn %d/10", i)
		}
		fmt.Println("")
		fmt.Println("Done")

		d := &devicePlugin.Device{}
		if err := d.Read(device); err != nil {
			if err := d.Create(device); err != nil {
				fmt.Println(err)
			}
		}

		fmt.Printf("starting to transform device configuration %s\n", d.Name)

		var interfaceList []*typesv1.Interface
		for _, activeInterfaceRefStatus := range device.Status.Interfaces {
			var activeInterface typesv1.Interface
			if err := client.Get(context.Background(), types.NamespacedName{Name: activeInterfaceRefStatus.InterfaceRef.Name, Namespace: activeInterfaceRefStatus.InterfaceRef.Namespace}, &activeInterface); err != nil {
				fmt.Println(err)
			} else if *activeInterfaceRefStatus.InterfaceRef.CommitStatus != typesv1.PENDINGDELETE {
				interfaceList = append(interfaceList, &activeInterface)
			}
		}
		result := d.ConfigureInterfaces(interfaceList)
		fmt.Println(result)
		updateStatus := false
		for intf, status := range result {
			if *status == typesv1.DELETESUCCESS {
				delete(device.Status.Interfaces, intf.Name)
				updateStatus = true
			} else if !(*status == typesv1.COMMITSUCCESS && *device.Status.Interfaces[intf.Name].InterfaceRef.CommitStatus == typesv1.COMMITSUCCESS) {
				device.Status.Interfaces[intf.Name].InterfaceRef.CommitStatus = status
				updateStatus = true
			}
		}
		if updateStatus {
			if err := client.Status().Update(context.Background(), device); err != nil {
				fmt.Println(err)
			}
		}
	}

}
