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
	_ = r.Log.WithValues("device", req.NamespacedName)
	r.Log.Info("Request for Device Controller ", "namespace/name", req.String())

	return ctrl.Result{}, nil
}

func (r *DeviceReconciler) setInterfaceFinalizer(interfaceStatus *devicemanagerv1.DeviceInterfaceStatus, deviceName string) error {
	var deviceInterface devicemanagerv1.Interface
	if err := r.Get(context.Background(), types.NamespacedName{Name: interfaceStatus.InterfaceRef.Name, Namespace: interfaceStatus.InterfaceRef.Namespace}, &deviceInterface); err != nil {
		r.Log.Error(err, "unable to fetch Interface")
		return err
	}

	if deviceInterface.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !containsString(deviceInterface.ObjectMeta.Finalizers, deviceName) {
			deviceInterface.ObjectMeta.Finalizers = append(deviceInterface.ObjectMeta.Finalizers, deviceName)
			if err := r.Update(context.Background(), &deviceInterface); err != nil {
				return err
			}
		}
	}
	return nil
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func (r *DeviceReconciler) interfaceDelete(actualInterfaceStatus *devicemanagerv1.DeviceInterfaceStatus, intendedInterfaceStatusList []*devicemanagerv1.DeviceInterfaceStatus) *devicemanagerv1.DeviceInterfaceStatus {
	interfaceInList := false
	for _, intendedInterfaceStatus := range intendedInterfaceStatusList {
		if actualInterfaceStatus.InterfaceRef.UID == intendedInterfaceStatus.InterfaceRef.UID {
			return nil
			//interfaceInList = true
			//break
		}
	}
	if !interfaceInList {
		pendingDelete := devicemanagerv1.PENDINGDELETE
		actualInterfaceStatus.InterfaceRef.CommitStatus = &pendingDelete
		return actualInterfaceStatus
		//intendedInterfaceStatusList = append(intendedInterfaceStatusList, actualInterfaceStatus)
		//fmt.Printf("blabla")
	}

	return nil
}

func (r *DeviceReconciler) interfaceCreateUpdate(intendedInterfaceStatus *devicemanagerv1.DeviceInterfaceStatus, actualInterfaceStatusList []*devicemanagerv1.DeviceInterfaceStatus) error {
	interfaceInList := false
	for _, actualInterfaceStatus := range actualInterfaceStatusList {
		if actualInterfaceStatus.InterfaceRef.UID == intendedInterfaceStatus.InterfaceRef.UID {
			interfaceInList = true
			actualInterfaceObj, err := r.getInterface(actualInterfaceStatus.InterfaceRef.Name, actualInterfaceStatus.InterfaceRef.Namespace)
			if err != nil {
				return err
			}
			intendedInterfaceObj, err := r.getInterface(intendedInterfaceStatus.InterfaceRef.Name, intendedInterfaceStatus.InterfaceRef.Namespace)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(actualInterfaceObj.Spec, intendedInterfaceObj.Spec) {
				pendingUpdate := devicemanagerv1.PENDINGUPDATE
				intendedInterfaceStatus.InterfaceRef.CommitStatus = &pendingUpdate
			} else {
				intendedInterfaceStatus.InterfaceRef.CommitStatus = actualInterfaceStatus.InterfaceRef.CommitStatus
			}
			return nil
		}
	}
	if !interfaceInList {
		pendingCreate := devicemanagerv1.PENDINGCREATE
		intendedInterfaceStatus.InterfaceRef.CommitStatus = &pendingCreate
	}
	return nil
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
		DeleteFunc: func(e event.DeleteEvent) bool {
			//return true
			deviceInterface, ok := e.Object.(*devicemanagerv1.Interface)
			if !ok {
				r.Log.Info("type conversion mismatch ", "type", deviceInterface.GetObjectKind())
			}
			return true
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
							q.Add(ctrl.Request{NamespacedName: types.NamespacedName{
								Name:      app.GetName(),
								Namespace: app.GetNamespace(),
							}})
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
