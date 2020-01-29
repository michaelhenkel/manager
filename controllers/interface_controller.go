/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	devicemanagerv1 "github.com/michaelhenkel/fabricmanager/api/v1"
)

// InterfaceReconciler reconciles a Interface object
type InterfaceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// Reconcile is the reconciler function
// +kubebuilder:rbac:groups=devicemanager.juniper.net,resources=interfaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=devicemanager.juniper.net,resources=interfaces/status,verbs=get;update;patch
func (r *InterfaceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("interface", req.NamespacedName)

	var deviceInterface devicemanagerv1.Interface
	if err := r.Get(context.Background(), req.NamespacedName, &deviceInterface); err != nil {
		r.Log.Error(err, "unable to fetch Device")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	/*
		deviceInterface.Status.ConfigHash = deviceInterface.Hash()
		if err := r.Status().Update(context.Background(), &deviceInterface); err != nil {
			r.Log.Error(err, "unable to update Device status")
			return ctrl.Result{}, err
		}
	*/

	return ctrl.Result{}, nil
}

func (r *InterfaceReconciler) interfaceRef(deviceInterface *devicemanagerv1.Interface) error {
	labelSelector := labels.SelectorFromSet(deviceInterface.GetLabels())
	listOps := &client.ListOptions{Namespace: deviceInterface.GetNamespace(), LabelSelector: labelSelector}
	list := &devicemanagerv1.DeviceList{}
	if err := r.List(context.Background(), list, listOps); err != nil {
		return err
	}
	if len(list.Items) > 0 {
		for _, device := range list.Items {
			for _, interfaceStatus := range device.Status.Interfaces {
				if interfaceStatus.InterfaceRef.UID == deviceInterface.GetUID() {
					return fmt.Errorf("Interface %s still referenced", device.Name)
				}
			}
		}
	}
	return nil
}

// SetupWithManager must be removed
func (r *InterfaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&devicemanagerv1.Interface{}).
		Complete(r)
}
