package main

import (
	"fmt"
	"net"
	"time"

	typesv1 "github.com/michaelhenkel/fabricmanager/api/v1"
)

// InterfaceHandler handles the interface configuration
func InterfaceHandler(activeInterface *typesv1.Interface) error {
	n := 2
	fmt.Printf("will finish in %d seconds \n", n)
	for _, unit := range activeInterface.Spec.Units {
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
			fmt.Printf("set interface %s unit %d family %s address %s\n", activeInterface.Spec.InterfaceIdentifier, unit.ID, family, address)
		}
	}
	time.Sleep(time.Duration(n) * time.Second)
	fmt.Println("done with transformation")
	return nil
}
