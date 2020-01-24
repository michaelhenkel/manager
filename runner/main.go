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
	"github.com/michaelhenkel/fabricmanager/runner/transformer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	kscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
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

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		fmt.Println(err)
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

	//deviceClient := &deviceClient{
	//	ns:         *namespacePtr,
	//	restClient: restClient,
	//}

	//deviceClient.Get("device1", metav1.GetOptions{})
	//fmt.Println(clientset)

	/*
		watchlist := cache.NewListWatchFromClient(
			//clientset.CoreV1().RESTClient(),
			restClient,
			"devices",
			//string(v1.ResourceServices),
			*namespacePtr,
			fields.Everything(),
		)
		_, controller := cache.NewInformer(
			watchlist,
			&typesv1.Device{},
			0, //Duration is int64
			cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					device, ok := obj.(*typesv1.Device)
					if !ok {
						fmt.Printf("Conversion failed: %s \n", obj)
					}
					if device.GetName() == *deviceNamePtr {
						fmt.Printf("Device added: %s \n", device.Status)
					}
				},
				DeleteFunc: func(obj interface{}) {
					device, ok := obj.(*typesv1.Device)
					if !ok {
						fmt.Printf("Conversion failed: %s \n", obj)
					}
					if device.GetName() == *deviceNamePtr {
						fmt.Printf("Device deleted: %s \n", device.Status)
					}
				},
				UpdateFunc: func(oldObj, newObj interface{}) {
					device, ok := newObj.(*typesv1.Device)
					if !ok {
						fmt.Printf("Conversion failed: %s \n", newObj)
					}
					if device.GetName() == *deviceNamePtr {
						fmt.Printf("Device Updated: %s \n", device.Status)
					}
				},
			},
		)

		stop := make(chan struct{})
		defer close(stop)
		go controller.Run(stop)
		for {
			time.Sleep(time.Second)
		}
	*/

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

	return nil
}

// Reconcile is the reconciler function
func (r *DeviceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	fmt.Printf("recon %s\n", req.NamespacedName)
	var device typesv1.Device
	if err := r.Get(context.Background(), req.NamespacedName, &device); err != nil {
		r.Log.Error(err, "unable to fetch Device")
	}
	transformer.Transform(&device, r.Client)
	return ctrl.Result{}, nil
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
				if !reflect.DeepEqual(oldDevice.Status, newDevice.Status) {
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
