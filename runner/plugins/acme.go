package plugin

import (
	"fmt"
	"net"
	"reflect"

	typesv1 "github.com/michaelhenkel/fabricmanager/api/v1"
)

type Device struct {
	*typesv1.Device
	Interfaces []*Interface
}

type Interface struct {
	*typesv1.Interface
}

func (p *Device) Create(deviceConfiguration *typesv1.Device) error {
	if _, ok := deviceMap[deviceConfiguration.GetName()]; !ok {
		//deviceConfiguration.DeepCopyInto(p.Device)
		p.Device = deviceConfiguration
		deviceMap[p.GetName()] = p
		fmt.Printf("Created new device %s \n", p.GetName())
		return nil
	}
	fmt.Printf("Device %s already exists \n", deviceConfiguration.GetName())
	return fmt.Errorf("Device already exits")
}

func (p *Device) Read(deviceConfiguration *typesv1.Device) error {
	if _, ok := deviceMap[deviceConfiguration.Name]; !ok {
		return fmt.Errorf("Device not found")
	}
	p.Device = deviceConfiguration
	p.Interfaces = deviceMap[deviceConfiguration.Name].Interfaces
	return nil
}

func (p *Device) Update(deviceConfiguration *typesv1.Device) error {
	if _, ok := deviceMap[deviceConfiguration.GetName()]; !ok {
		return fmt.Errorf("Device not found")
	}
	if !reflect.DeepEqual(p.Device, deviceConfiguration) {
		p.Device = deviceConfiguration
		p.Interfaces = deviceMap[deviceConfiguration.GetName()].Interfaces
		fmt.Printf("Updated device %s\n", p.GetName())
	}
	return nil
}

func (p *Device) ConfigureInterfaces(interfaceList []*typesv1.Interface) map[*typesv1.Interface]*typesv1.CommitStatus {
	var interfaceStatusMap = make(map[*typesv1.Interface]*typesv1.CommitStatus)

	newInterfaceList := newInterfaces(interfaceList, p.Interfaces)
	fmt.Println(newInterfaceList)
	for _, intendedIntf := range newInterfaceList {
		var commitStatus typesv1.CommitStatus
		actIntf := &Interface{}
		if err := actIntf.Create(intendedIntf, p); err == nil {
			commitStatus = typesv1.COMMITSUCCESS
			deviceMap[p.GetName()] = p
		} else {
			commitStatus = typesv1.CREATEFAIL
		}
		interfaceStatusMap[actIntf.Interface] = &commitStatus
	}

	updateInterfaceList := updateInterfaces(interfaceList, p.Interfaces)
	fmt.Println(updateInterfaceList)
	for _, intendedIntf := range updateInterfaceList {
		var commitStatus typesv1.CommitStatus
		actIntf := &Interface{}
		if err := actIntf.Update(intendedIntf, p); err == nil {
			commitStatus = typesv1.COMMITSUCCESS
			deviceMap[p.GetName()] = p
		} else {
			commitStatus = typesv1.UPDATEFAIL
		}
		interfaceStatusMap[actIntf.Interface] = &commitStatus
	}

	var deleteIdx []int
	deleteInterfaceList := deleteInterfaces(p.Interfaces, interfaceList)
	fmt.Println(deleteInterfaceList)
	for idx, deleteIntf := range deleteInterfaceList {
		var commitStatus typesv1.CommitStatus
		if err := deleteIntf.Delete(p); err == nil {
			commitStatus = typesv1.DELETESUCCESS
		} else {
			commitStatus = typesv1.DELETEFAIL
		}
		interfaceStatusMap[deleteIntf.Interface] = &commitStatus
		deleteIdx = append(deleteIdx, idx)
	}

	if len(deleteIdx) > 0 {
		for _, deleteIdx := range deleteIdx {
			p.Interfaces = append(p.Interfaces[:deleteIdx], p.Interfaces[deleteIdx+1:]...)
			deleteIdx--
		}
	}

	for _, intf := range interfaceList {
		if _, ok := interfaceStatusMap[intf]; !ok {
			commitStatus := typesv1.COMMITSUCCESS
			interfaceStatusMap[intf] = &commitStatus
		}
	}
	return interfaceStatusMap
}

func newInterfaces(a []*typesv1.Interface, b []*Interface) []*typesv1.Interface {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x.Interface.Name] = struct{}{}
	}
	var diff []*typesv1.Interface
	for _, x := range a {
		if _, found := mb[x.Name]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

func updateInterfaces(a []*typesv1.Interface, b []*Interface) []*typesv1.Interface {
	var diff []*typesv1.Interface
	for _, intendedItf := range a {
		for _, actualItf := range b {
			if intendedItf.Name == actualItf.GetName() {
				if !reflect.DeepEqual(intendedItf, actualItf.Interface) {
					diff = append(diff, intendedItf)
				}
			}
		}
	}
	return diff
}

func deleteInterfaces(a []*Interface, b []*typesv1.Interface) []*Interface {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x.Name] = struct{}{}
	}
	var diff []*Interface
	for _, x := range a {
		if _, found := mb[x.Interface.Name]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

func (i *Interface) Create(intendedInterface *typesv1.Interface, device *Device) error {
	i.Interface = intendedInterface
	device.Interfaces = append(device.Interfaces, i)
	fmt.Printf("adding Interface %s to Device %s\n", i.Name, device.Name)
	action := "set"
	for _, unit := range i.Spec.Units {
		for _, address := range unit.Addresses {
			ip, _, err := net.ParseCIDR(address)
			if err != nil {
				return err
			}
			var family string
			if ip.To4() != nil {
				family = "inet"
			} else {
				family = "inet6"
			}
			fmt.Printf("%s interface %s unit %d family %s address %s\n", action, i.Spec.InterfaceIdentifier, unit.ID, family, address)
		}
	}
	return nil
}

func (i *Interface) Update(intendedInterface *typesv1.Interface, device *Device) error {
	i.Interface = intendedInterface
	fmt.Printf("updating Interface %s on Device %s\n", i.GetName(), device.GetName())
	action := "update"
	for _, unit := range i.Spec.Units {
		for _, address := range unit.Addresses {
			ip, _, err := net.ParseCIDR(address)
			if err != nil {
				return err
			}
			var family string
			if ip.To4() != nil {
				family = "inet"
			} else {
				family = "inet6"
			}
			fmt.Printf("%s interface %s unit %d family %s address %s\n", action, i.Spec.InterfaceIdentifier, unit.ID, family, address)
		}
	}
	return nil
}

func (i *Interface) Delete(device *Device) error {
	action := "delete"
	fmt.Printf("%s interface %s\n", action, i.Spec.InterfaceIdentifier)
	return nil
}

var deviceMap = make(map[string]*Device)
