package transformer

import (
	"context"
	"fmt"

	typesv1 "github.com/michaelhenkel/fabricmanager/api/v1"
	devicePlugin "github.com/michaelhenkel/fabricmanager/runner/plugins"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Transform transforms a device configuration
func Transform(device *typesv1.Device, client client.Client) {

	d := &devicePlugin.Device{}
	if err := d.Read(device); err != nil {
		if err := d.Create(device); err != nil {
			fmt.Println(err)
		}
	}

	fmt.Printf("starting to transform device configuration %s\n", d.Name)
	/*
		rand.Seed(time.Now().UnixNano())
		n := rand.Intn(20) // n will be between 0 and 10
	*/
	/*
		pluginPath := "plugins/" + device.Spec.Vendor + ".so"
		p, err := plugin.Open(pluginPath)
		if err != nil {
			panic(err)
		}

		interfaceHandler, err := p.Lookup("InterfaceHandler")
		if err != nil {
			panic(err)
		}*/
	var interfaceList []*typesv1.Interface
	for _, activeInterfaceRefStatus := range device.Status.Interfaces {
		var activeInterface typesv1.Interface
		if err := client.Get(context.Background(), types.NamespacedName{Name: activeInterfaceRefStatus.InterfaceRef.Name, Namespace: activeInterfaceRefStatus.InterfaceRef.Namespace}, &activeInterface); err != nil {
			fmt.Println(err)
		} else {
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
		} else if !(*status == typesv1.CREATESUCCESS && *device.Status.Interfaces[intf.Name].InterfaceRef.CommitStatus == typesv1.CREATESUCCESS) {
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

func removeFinalizer(activeInterface *typesv1.Interface, deviceName string, client client.Client) error {
	if containsString(activeInterface.ObjectMeta.Finalizers, deviceName) {
		activeInterface.ObjectMeta.Finalizers = removeString(activeInterface.ObjectMeta.Finalizers, deviceName)
		if err := client.Update(context.Background(), activeInterface); err != nil {
			return err
		}
	}
	return nil
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
