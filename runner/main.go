package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/go-logr/logr"
	typesv1 "github.com/michaelhenkel/fabricmanager/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	kscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// KubeClientConfig defines the
type KubeClientConfig struct {
	InCluster  bool
	ConfigPath string
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = typesv1.AddToScheme(scheme)
}

//go:generate go build -buildmode=plugin -o plugins/acme.so plugins/acme.go
func main() {
	var metricsAddr string
	var enableLeaderElection bool
	configPathPtr := flag.String("configpath", "", "path to config yaml file")
	inclusterPtr := flag.Bool("incluster", false, "path to config yaml file")
	namespacePtr := flag.String("namespace", "default", "path to config yaml file")
	deviceNamePtr := flag.String("devicename", "default", "path to config yaml file")
	flag.StringVar(&metricsAddr, "metrics-addr", ":8099", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	kubeClientConfig := KubeClientConfig{
		InCluster:  *inclusterPtr,
		ConfigPath: *configPathPtr,
	}
	_, _, restConfig, err := kubeClient(kubeClientConfig)
	if err != nil {
		panic(err)
	}

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     enableLeaderElection,
		Port:               9443,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&DeviceReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Device"),
		Scheme: mgr.GetScheme(),
	}).Add(mgr, *deviceNamePtr, *namespacePtr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Device")
		os.Exit(1)
	}
	setupLog.Info("Started controller", "controller", *deviceNamePtr)

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		fmt.Println(err)
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// DeviceReconciler reconciles a Device object
type DeviceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// Add adds the controller to the manager
func (r *DeviceReconciler) Add(mgr ctrl.Manager, devicename string, namespace string) error {

	c, err := controller.New("device-controller", mgr,
		controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	deviceSrc := &source.Kind{Type: &typesv1.Device{}}
	devicePredicate := r.Change(devicename, namespace)
	if err = c.Watch(deviceSrc, &handler.EnqueueRequestForObject{}, devicePredicate); err != nil {
		return err
	}

	interfaceSrc := &source.Kind{Type: &typesv1.Interface{}}
	interfaceHandler := r.resourceHandler()
	interfacePredicate := r.InterfaceChange()
	if err = c.Watch(interfaceSrc, interfaceHandler, interfacePredicate); err != nil {
		return err
	}

	return nil
}

// InterfaceChange creates predicates when the interface changes.
func (r *DeviceReconciler) InterfaceChange() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			//return true
			oldInterface, ok := e.ObjectOld.(*typesv1.Interface)
			if !ok {
				r.Log.Info("type conversion mismatch ", "type", oldInterface.GetObjectKind())
			}

			newInterface, ok := e.ObjectNew.(*typesv1.Interface)
			if !ok {
				r.Log.Info("type conversion mismatch ", "type", newInterface.GetObjectKind())
			}
			if !reflect.DeepEqual(oldInterface.Spec, newInterface.Spec) {
				r.Log.Info("Update Event ", "type", oldInterface.GetObjectKind())
				return true
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			//return true
			deviceInterface, ok := e.Object.(*typesv1.Interface)
			if !ok {
				r.Log.Info("type conversion mismatch ", "type", deviceInterface.GetObjectKind())
			}
			r.Log.Info("Delete Event ", "type", deviceInterface.GetObjectKind())
			return true
		},
	}
}

func (r *DeviceReconciler) resourceHandler() handler.Funcs {
	appHandler := handler.Funcs{
		CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
			listOps := &client.ListOptions{Namespace: e.Meta.GetNamespace()}
			list := &typesv1.DeviceList{}
			err := r.List(context.Background(), list, listOps)
			if err == nil {
				for _, app := range list.Items {
					for _, interfaceSelector := range app.Spec.InterfaceSelector {
						labelSelector := labels.SelectorFromSet(interfaceSelector)
						listOps := &client.ListOptions{Namespace: e.Meta.GetNamespace(), LabelSelector: labelSelector}
						list := &typesv1.InterfaceList{}
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
			list := &typesv1.DeviceList{}
			err := r.List(context.Background(), list, listOps)
			if err == nil {
				for _, app := range list.Items {
					for _, interfaceSelector := range app.Spec.InterfaceSelector {
						labelSelector := labels.SelectorFromSet(interfaceSelector)
						listOps := &client.ListOptions{Namespace: e.MetaNew.GetNamespace(), LabelSelector: labelSelector}
						list := &typesv1.InterfaceList{}
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
			list := &typesv1.DeviceList{}
			err := r.List(context.Background(), list, listOps)
			if err == nil {
				for _, app := range list.Items {
					for _, interfaceSelector := range app.Spec.InterfaceSelector {
						labelSelector := labels.SelectorFromSet(interfaceSelector)
						listOps := &client.ListOptions{Namespace: e.Meta.GetNamespace(), LabelSelector: labelSelector}
						list := &typesv1.InterfaceList{}
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
			list := &typesv1.DeviceList{}
			err := r.List(context.Background(), list, listOps)
			if err == nil {
				for _, app := range list.Items {
					for _, interfaceSelector := range app.Spec.InterfaceSelector {
						labelSelector := labels.SelectorFromSet(interfaceSelector)
						listOps := &client.ListOptions{Namespace: e.Meta.GetNamespace(), LabelSelector: labelSelector}
						list := &typesv1.InterfaceList{}
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

// Reconcile is the reconciler function
func (r *DeviceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	fmt.Printf("recon %s\n", req.NamespacedName)
	var device typesv1.Device
	if err := r.Get(context.Background(), req.NamespacedName, &device); err != nil {
		r.Log.Error(err, "unable to fetch Device")
	}

	var intendedInterfaceStatusMap = make(map[string]*typesv1.DeviceInterfaceStatus)
	if len(device.Spec.InterfaceSelector) > 0 {
		for _, interfaceMap := range device.Spec.InterfaceSelector {
			labelSelector := labels.SelectorFromSet(interfaceMap)
			listOps := &client.ListOptions{Namespace: req.Namespace, LabelSelector: labelSelector}
			list := &typesv1.InterfaceList{}
			if err := r.List(context.Background(), list, listOps); err != nil {
				return ctrl.Result{}, err
			}
			for _, interfaceObj := range list.Items {
				interfaceObj.GetReference()
				interfaceRef := interfaceObj.GetReference()
				deviceInterfaceStatus := &typesv1.DeviceInterfaceStatus{
					InterfaceRef: interfaceRef,
				}
				intendedInterfaceStatusMap[interfaceRef.Name] = deviceInterfaceStatus
			}
		}
	}
	actualInterfaceStatusMap := device.Status.Interfaces

	update := false

	// Update
	for _, intendedInterfaceStatus := range intendedInterfaceStatusMap {
		if actualInterfaceStatus, ok := actualInterfaceStatusMap[intendedInterfaceStatus.InterfaceRef.Name]; ok {
			res, err := r.interfaceUpdate(intendedInterfaceStatus, actualInterfaceStatus)
			if err != nil {
				return ctrl.Result{}, err
			}
			if res != nil && *res {
				update = true
			}
		}
	}

	// Create
	for _, intendedInterfaceStatus := range intendedInterfaceStatusMap {
		if _, ok := actualInterfaceStatusMap[intendedInterfaceStatus.InterfaceRef.Name]; !ok {
			r.interfaceCreate(intendedInterfaceStatus)
			update = true
		} else if *actualInterfaceStatusMap[intendedInterfaceStatus.InterfaceRef.Name].InterfaceRef.CommitStatus == typesv1.PENDINGDELETE {
			r.interfaceCreate(intendedInterfaceStatus)
			update = true
		}
	}

	// Delete
	for _, actualInterfaceStatus := range actualInterfaceStatusMap {
		if _, ok := intendedInterfaceStatusMap[actualInterfaceStatus.InterfaceRef.Name]; !ok {
			newInterfaceStatus := r.interfaceDelete(actualInterfaceStatus, intendedInterfaceStatusMap)
			intendedInterfaceStatusMap[newInterfaceStatus.InterfaceRef.Name] = newInterfaceStatus
			update = true
		}
	}

	if update {
		device.Status.Interfaces = intendedInterfaceStatusMap
		if err := r.Status().Update(context.Background(), &device); err != nil {
			r.Log.Error(err, "unable to update Device status")
			return ctrl.Result{}, err
		}
	}

	//transformer.Transform(&device, r.Client)
	return ctrl.Result{}, nil
}

func (r *DeviceReconciler) interfaceDelete(actualInterfaceStatus *typesv1.DeviceInterfaceStatus, intendedInterfaceStatusMap map[string]*typesv1.DeviceInterfaceStatus) *typesv1.DeviceInterfaceStatus {
	pendingDelete := typesv1.PENDINGDELETE
	actualInterfaceStatus.InterfaceRef.CommitStatus = &pendingDelete
	return actualInterfaceStatus
}

func (r *DeviceReconciler) interfaceCreate(intendedInterfaceStatus *typesv1.DeviceInterfaceStatus) {
	pendingCreate := typesv1.PENDINGCREATE
	intendedInterfaceStatus.InterfaceRef.CommitStatus = &pendingCreate
}

func (r *DeviceReconciler) interfaceUpdate(intendedInterfaceStatus *typesv1.DeviceInterfaceStatus, actualInterfaceStatus *typesv1.DeviceInterfaceStatus) (*bool, error) {
	actualInterfaceObj, err := r.getInterface(actualInterfaceStatus.InterfaceRef.Name, actualInterfaceStatus.InterfaceRef.Namespace)
	if err != nil {
		return nil, err
	}
	actualHash := actualInterfaceObj.Hash()
	if actualHash != actualInterfaceStatus.InterfaceRef.ConfigHash {
		pendingUpdate := typesv1.PENDINGUPDATE
		intendedInterfaceStatus.InterfaceRef.CommitStatus = &pendingUpdate
		update := true
		return &update, nil
	}

	intendedInterfaceStatus.InterfaceRef.CommitStatus = actualInterfaceStatus.InterfaceRef.CommitStatus

	return nil, nil
}

func (r *DeviceReconciler) getInterface(name string, namespace string) (*typesv1.Interface, error) {
	ctx := context.Background()
	interfaceObj := &typesv1.Interface{}
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, interfaceObj); err != nil {
		return interfaceObj, err
	}
	return interfaceObj, nil
}

// Change creates predicates when the device changes.
func (r *DeviceReconciler) Change(deviceName string, namespace string) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if deviceName == e.Meta.GetName() && namespace == e.Meta.GetNamespace() {
				return true
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			if deviceName == e.Meta.GetName() && namespace == e.Meta.GetNamespace() {
				return true
			}
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if deviceName == e.MetaNew.GetName() && namespace == e.MetaNew.GetNamespace() {
				oldDevice, ok := e.ObjectOld.(*typesv1.Device)
				if !ok {
					r.Log.Info("type conversion mismatch ", "type", oldDevice.GetObjectKind())
				}
				newDevice, ok := e.ObjectNew.(*typesv1.Device)
				if !ok {
					r.Log.Info("type conversion mismatch ", "type", newDevice.GetObjectKind())
				}
				//if !reflect.DeepEqual(oldDevice.Spec, newDevice.Spec) || !reflect.DeepEqual(oldDevice.Status, newDevice.Status) {
				if !reflect.DeepEqual(oldDevice.Spec, newDevice.Spec) {
					return true
				}
			}
			return false
		},
	}
}

type deviceClient struct {
	restClient rest.Interface
	ns         string
}

func (c *deviceClient) Get(name string, opts metav1.GetOptions) (*typesv1.Device, error) {
	result := typesv1.Device{}
	err := c.restClient.
		Get().
		Namespace(c.ns).
		Resource("controls").
		Name(name).
		VersionedParams(&opts, kscheme.ParameterCodec).
		Do().
		Into(&result)

	return &result, err
}

func kubeClient(kubeClientConfig KubeClientConfig) (*kubernetes.Clientset, *rest.RESTClient, *rest.Config, error) {
	var err error
	clientset := &kubernetes.Clientset{}
	restClient := &rest.RESTClient{}
	kubeConfig := &rest.Config{}
	if !kubeClientConfig.InCluster {
		var kubeConfigPath string
		if kubeClientConfig.ConfigPath != "" {
			kubeConfigPath = kubeClientConfig.ConfigPath
		} else {
			kubeConfigPath = filepath.Join(homeDir(), ".kube", "config")
		}
		kubeConfig, err = clientcmd.BuildConfigFromFlags("", kubeConfigPath)
		if err != nil {
			return clientset, restClient, &rest.Config{}, err
		}

	} else {
		kubeConfig, err = rest.InClusterConfig()
		if err != nil {
			return clientset, restClient, &rest.Config{}, err
		}
		kubeConfig.CAFile = ""
		kubeConfig.TLSClientConfig.Insecure = true
	}
	// create the clientset
	typesv1.SchemeBuilder.AddToScheme(kscheme.Scheme)

	crdConfig := kubeConfig
	gv := typesv1.SchemeBuilder.GroupVersion
	crdConfig.ContentConfig.GroupVersion = &schema.GroupVersion{Group: gv.Group, Version: gv.Version}

	crdConfig.APIPath = "/apis"

	crdConfig.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: kscheme.Codecs}
	crdConfig.UserAgent = rest.DefaultKubernetesUserAgent()

	restClient, err = rest.UnversionedRESTClientFor(crdConfig)
	if err != nil {
		return clientset, restClient, &rest.Config{}, err
	}
	clientset, err = kubernetes.NewForConfig(crdConfig)
	if err != nil {
		return clientset, restClient, &rest.Config{}, err
	}
	return clientset, restClient, crdConfig, nil
}
