package watcher

import (
	"context"
	"encoding/json"
	"fmt"


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
	//"k8s.io/apimachinery/pkg/watch"
)

var log = logf.Log.WithName("controller_watcher")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

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

//var commonInformerFactory = &InformerFactory{}
var commonInformerFactory *dynamicinformer.DynamicSharedInformerFactory

var allInformersFactories = make(map[string]*dynamicinformer.DynamicSharedInformerFactory)

func GetInformerFactoryForNamespace(namespace string) dynamicinformer.DynamicSharedInformerFactory {
	value, found := allInformersFactories[namespace]
	if found {
		return *value
	}

	if commonInformerFactory == nil {
		config, err := config.GetConfig()

		// Grab a dynamic interface that we can create informers from
		dynamicset, err := dynamic.NewForConfig(config)
		if err != nil {
			fmt.Printf("could not generate dynamic client for config")
		}

		f := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynamicset, 0, namespace, nil)
		allInformersFactories[namespace] = &f
		//informerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynamicset, 0, request.Namespace, nil)
	}
	return *allInformersFactories[namespace]
}

//Reconcile reads that state of the cluster for a Watcher object and makes changes based on the state read
//and what is in the Watcher.Spec
func (r *ReconcileWatcher) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Basic: Сделать структуру с указателем на InformerFactory + канал. Тут каждый раз убивать канал, создавать новый объект
	// с InformerFactory, менять глобальный указатель на эту структуру, тогда получается и канал в ней будет новый и можно
	// создавать информеры заново. Advanced: подумать не будет ли какой-то жести при куче вызовов reconcile параллельно
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Watcher")
	// Fetch the Watcher instance
	instance := &aisonakuv1alpha1.Watcher{}
	//fmt.Println(instance)
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	informerFactory := GetInformerFactoryForNamespace(request.Namespace)

	// Retrieve a "GroupVersionResource" type that we need when generating our informer from our dynamic factory
	gvr1, _ := schema.ParseResourceArg(instance.Spec.Kind)
	//gvr2, _ := schema.ParseResourceArg("events.v1.")

	// Finally, create our informers!
	informer1 := informerFactory.ForResource(*gvr1)
	fmt.Println(informerFactory)

	stopCh := make(chan struct{})
	go startWatching(stopCh, informer1.Informer())

	return reconcile.Result{}, nil
}

func startWatching(stopCh <-chan struct{}, s cache.SharedIndexInformer) {
	handlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			jsonString, _ := json.Marshal(u)
			fmt.Printf("resource added: %s \n", jsonString)
		},
		DeleteFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			fmt.Printf("resource deleted: %s \n", u)
		},
	}

	s.AddEventHandler(handlers)
	s.Run(stopCh)
}