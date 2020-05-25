package watcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	aisonakuv1alpha1 "watcher-operator/pkg/apis/aisonaku/v1alpha1"

	//corev1 "k8s.io/api/core/v1"
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/discovery"
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//"k8s.io/apimachinery/pkg/watch"
)

var log = logf.Log.WithName("controller_watcher")


// Add creates a new Watcher Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileWatcher{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("watcher-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Watcher
	err = c.Watch(&source.Kind{Type: &aisonakuv1alpha1.Watcher{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}
	return nil
}

// blank assignment to verify that ReconcileWatcher implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileWatcher{}

// ReconcileWatcher reconciles a Watcher object
type ReconcileWatcher struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

type InfomerFactory struct {
	factory dynamicinformer.DynamicSharedInformerFactory
	stopCh chan struct{}
}

var allInformersFactories = make(map[string]*InfomerFactory)

func GetInformerFactoryForNamespace(namespace string) (InfomerFactory, bool) {
	value, found := allInformersFactories[namespace]
	if found {
		return *value, false
	}

	config, err := config.GetConfig()

	// Grab a dynamic interface that we can create informers from
	dynamicset, err := dynamic.NewForConfig(config)
	if err != nil {
		fmt.Printf("could not generate dynamic client for config")
	}

	f := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynamicset, 0, namespace, nil)
	newInformer := &InfomerFactory{factory: f, stopCh: make(chan struct{})}
	allInformersFactories[namespace] = newInformer

	return *allInformersFactories[namespace], true
}

//Reconcile reads that state of the cluster for a Watcher object and makes changes based on the state read
//and what is in the Watcher.Spec
func (r *ReconcileWatcher) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// подумать не будет ли какой-то жести при куче вызовов reconcile параллельно и изменеия переменной allInformersFactories
	namespace := request.Namespace

	reqLogger := log.WithValues("Request.Namespace", namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Watcher")
	// Fetch the Watcher instance
	instance := &aisonakuv1alpha1.Watcher{}
	informerFactoryType, isNew := GetInformerFactoryForNamespace(namespace)
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			close(informerFactoryType.stopCh)
			delete(allInformersFactories, namespace)
			fmt.Println("Object Watch not found fot namespace " + namespace)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		fmt.Println("Error reading the object - requeue the request")
		return reconcile.Result{}, err
	}


	// Stop working informers for namespace to create the updated versions
	if !isNew {
		close(informerFactoryType.stopCh)
		delete(allInformersFactories, namespace)
		informerFactoryType, _ = GetInformerFactoryForNamespace(namespace)
	}

	informerFactory := informerFactoryType.factory

	watchResources := instance.Spec.WatchResources
	//fmt.Println(watchResources)

	for _, resource := range watchResources {
		// Retrieve a "GroupVersionResource" type that we need when generating our informer from our dynamic factory
		gvr, _ := DetermineGroupVersionResource(resource)
		reqLogger.Info("Start watching " + gvr.String())
		informerFactory.ForResource(gvr)
		//Finally, create an informer!
		informer := informerFactory.ForResource(gvr)

		go startWatching(informerFactoryType.stopCh, informer.Informer(), instance.Spec.EventListener)
	}


	return reconcile.Result{}, nil
}

func startWatching(stopCh <-chan struct{}, s cache.SharedIndexInformer, eventListenerUrl string) {
	handlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			jsonString, _ := json.Marshal(u)
			fmt.Printf("resource added: %s \n", jsonString)
		},
		UpdateFunc: func(oldObj, obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			jsonString, _ := json.Marshal(u)
			fmt.Printf("resource updated: %s \n", jsonString)
		},
		DeleteFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			jsonString, _ := json.Marshal(u)
			fmt.Printf("resource deleted: %s \n", jsonString)
			body := strings.NewReader(string(jsonString))
			fmt.Println(os.Getenv("EL_URL"))
			req, err := http.NewRequest("POST", eventListenerUrl, body)
			if err != nil {
				// handle err
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				// handle err
			}
			defer resp.Body.Close()
		},
	}

	s.AddEventHandler(handlers)
	s.Run(stopCh)
}

func DetermineGroupVersionResource(watchResource aisonakuv1alpha1.WatchResource) (schema.GroupVersionResource, error) {
	// Resolve resource kind to the underlying API Resource type.
	apiResource, err := FindAPIResource(watchResource.ApiVersion)
	if err != nil {
		fmt.Println(err)
		return schema.GroupVersionResource{}, err
	}

	gvr := apiResource.WithResource(watchResource.Resource)
	return gvr, nil
}

func FindAPIResource(apiVersion string) (schema.GroupVersion, error) {
	c, err := config.GetConfig()

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(c)
	if err != nil {
		fmt.Printf("could not generate discovery client for config")
	}

	resourceList, err := discoveryClient.ServerResourcesForGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersion{}, fmt.Errorf("error getting kubernetes server resources for apiVersion %s: %s", apiVersion, err)
	}
	for _, apiResource := range resourceList.APIResources {
		r := &apiResource
		gv := schema.GroupVersion{}
		// Resolve GroupVersion from parent list to have consistent resource identifiers.
		if r.Version == "" || r.Group == "" {
			gv, err = schema.ParseGroupVersion(resourceList.GroupVersion)
			if err != nil {
				return schema.GroupVersion{}, fmt.Errorf("error parsing parsing GroupVersion: %v", err)
			}
		}
		return gv, nil
	}
	return schema.GroupVersion{}, fmt.Errorf("error could not find resource with apiVersion", apiVersion)
}