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
	"reflect"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ref "k8s.io/client-go/tools/reference"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"sigs.k8s.io/controller-runtime/pkg/controller"

	devicemanagerv1 "github.com/michaelhenkel/fabricmanager/api/v1"
)

// DeviceReconciler reconciles a Device object
type DeviceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// Add adds the controller to the manager
func (r *DeviceReconciler) Add(mgr ctrl.Manager) error {

	c, err := controller.New("device-controller", mgr,
		controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{Type: &devicemanagerv1.Device{}},
		&handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	interfaceSrc := &source.Kind{Type: &devicemanagerv1.Interface{}}
	interfaceHandler := r.resourceHandler()
	interfacePredicate := r.InterfaceChange()
	if err = c.Watch(interfaceSrc, interfaceHandler, interfacePredicate); err != nil {
		return err
	}

	return nil
}

// Reconcile is the reconciler function
// ||||+kubebuilder:rbac:groups=devicemanager.juniper.net,resources=interfaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=devicemanager.juniper.net,resources=devices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=devicemanager.juniper.net,resources=devices/status,verbs=get;update;patch
func (r *DeviceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("device", req.NamespacedName)
	r.Log.Info("Request for Device Controller ", "namespace/name", req.String())

	var device devicemanagerv1.Device
	if err := r.Get(ctx, req.NamespacedName, &device); err != nil {
		r.Log.Error(err, "unable to fetch Device")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var activeInterfaceStatusList []*devicemanagerv1.DeviceInterfaceStatus
	if len(device.Spec.InterfaceSelector) > 0 {
		for _, interfaceMap := range device.Spec.InterfaceSelector {
			labelSelector := labels.SelectorFromSet(interfaceMap)
			listOps := &client.ListOptions{Namespace: req.Namespace, LabelSelector: labelSelector}
			list := &devicemanagerv1.InterfaceList{}
			if err := r.List(context.Background(), list, listOps); err != nil {
				return ctrl.Result{}, err
			}
			for _, interfaceObj := range list.Items {
				interfaceRef, err := ref.GetReference(r.Scheme, &interfaceObj)
				if err != nil {
					r.Log.Error(err, "unable to get Interface reference")
					return ctrl.Result{}, err
				}
				pending := devicemanagerv1.PENDING
				deviceInterfaceStatus := devicemanagerv1.DeviceInterfaceStatus{
					InterfaceRef: interfaceRef,
					CommitStatus: &pending,
				}
				activeInterfaceStatusList = append(activeInterfaceStatusList, &deviceInterfaceStatus)
			}
		}
	}

	deviceUpdate := false
	for _, activeInterfaceStatus := range activeInterfaceStatusList {
		for _, interfaceStatus := range device.Status.Interfaces {
			if activeInterfaceStatus.InterfaceRef == interfaceStatus.InterfaceRef {
				if activeInterfaceStatus.CommitStatus != interfaceStatus.CommitStatus {
					activeInterfaceStatus.CommitStatus = interfaceStatus.CommitStatus
					deviceUpdate = true
				}

			}
		}

	}
	if deviceUpdate {
		device.Status.Interfaces = activeInterfaceStatusList
		if err := r.Status().Update(ctx, &device); err != nil {
			r.Log.Error(err, "unable to update Device status")
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

var (
	jobOwnerKey = ".metadata.controller"
	nameKey     = ".metadata.name"
	apiGVStr    = devicemanagerv1.GroupVersion.String()
)

// InterfaceChange creates predicates when the interface changes.
func (r *DeviceReconciler) InterfaceChange() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			//return true
			oldInterface, ok := e.ObjectOld.(*devicemanagerv1.Interface)
			if !ok {
				r.Log.Info("type conversion mismatch ", "type", oldInterface.GetObjectKind())
			}
			newInterface, ok := e.ObjectNew.(*devicemanagerv1.Interface)
			if !ok {
				r.Log.Info("type conversion mismatch ", "type", newInterface.GetObjectKind())
			}
			if !reflect.DeepEqual(oldInterface, newInterface) {
				return true
			}
			return false
		},
	}
}

func (r *DeviceReconciler) getInterface(name string, namespace string) (*devicemanagerv1.Interface, error) {
	ctx := context.Background()
	interfaceObj := &devicemanagerv1.Interface{}
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, interfaceObj); err != nil {
		return interfaceObj, err
	}
	return interfaceObj, nil
}

func (r *DeviceReconciler) resourceHandler() handler.Funcs {
	appHandler := handler.Funcs{
		CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
			listOps := &client.ListOptions{Namespace: e.Meta.GetNamespace()}
			list := &devicemanagerv1.DeviceList{}
			err := r.List(context.Background(), list, listOps)
			if err == nil {
				for _, app := range list.Items {
					for _, interfaceSelector := range app.Spec.InterfaceSelector {
						labelSelector := labels.SelectorFromSet(interfaceSelector)
						listOps := &client.ListOptions{Namespace: e.Meta.GetNamespace(), LabelSelector: labelSelector}
						list := &devicemanagerv1.InterfaceList{}
						if err := r.List(context.Background(), list, listOps); err == nil {
							if len(list.Items) > 0 {
								q.Add(ctrl.Request{NamespacedName: types.NamespacedName{
									Name:      app.GetName(),
									Namespace: e.Meta.GetNamespace(),
								}})
							}
						}
					}
				}
			}
		},
		UpdateFunc: func(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
			listOps := &client.ListOptions{Namespace: e.MetaNew.GetNamespace()}
			list := &devicemanagerv1.DeviceList{}
			err := r.List(context.Background(), list, listOps)
			if err == nil {
				for _, app := range list.Items {
					for _, interfaceSelector := range app.Spec.InterfaceSelector {
						labelSelector := labels.SelectorFromSet(interfaceSelector)
						listOps := &client.ListOptions{Namespace: e.MetaNew.GetNamespace(), LabelSelector: labelSelector}
						list := &devicemanagerv1.InterfaceList{}
						if err := r.List(context.Background(), list, listOps); err == nil {
							if len(list.Items) > 0 {
								fmt.Println("DEVICE UPDATE")
								q.Add(ctrl.Request{NamespacedName: types.NamespacedName{
									Name:      app.GetName(),
									Namespace: e.MetaNew.GetNamespace(),
								}})
							}
						}
					}
				}
			}
		},
		DeleteFunc: func(e event.DeleteEvent, q workqueue.RateLimitingInterface) {
			listOps := &client.ListOptions{Namespace: e.Meta.GetNamespace()}
			list := &devicemanagerv1.DeviceList{}
			err := r.List(context.Background(), list, listOps)
			if err == nil {
				for _, app := range list.Items {
					for _, interfaceSelector := range app.Spec.InterfaceSelector {
						labelSelector := labels.SelectorFromSet(interfaceSelector)
						listOps := &client.ListOptions{Namespace: e.Meta.GetNamespace(), LabelSelector: labelSelector}
						list := &devicemanagerv1.InterfaceList{}
						if err := r.List(context.Background(), list, listOps); err == nil {
							if len(list.Items) > 0 {
								q.Add(ctrl.Request{NamespacedName: types.NamespacedName{
									Name:      app.GetName(),
									Namespace: e.Meta.GetNamespace(),
								}})
							}
						}
					}
				}
			}
		},
		GenericFunc: func(e event.GenericEvent, q workqueue.RateLimitingInterface) {
			listOps := &client.ListOptions{Namespace: e.Meta.GetNamespace()}
			list := &devicemanagerv1.DeviceList{}
			err := r.List(context.Background(), list, listOps)
			if err == nil {
				for _, app := range list.Items {
					for _, interfaceSelector := range app.Spec.InterfaceSelector {
						labelSelector := labels.SelectorFromSet(interfaceSelector)
						listOps := &client.ListOptions{Namespace: e.Meta.GetNamespace(), LabelSelector: labelSelector}
						list := &devicemanagerv1.InterfaceList{}
						if err := r.List(context.Background(), list, listOps); err == nil {
							if len(list.Items) > 0 {
								q.Add(ctrl.Request{NamespacedName: types.NamespacedName{
									Name:      app.GetName(),
									Namespace: e.Meta.GetNamespace(),
								}})
							}
						}
					}
				}
			}
		},
	}
	return appHandler
}
