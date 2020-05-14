package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	//"time"

	//"github.com/golang/glog"

	//kubeinformers "k8s.io/client-go/informers"
	//"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	//v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func main() {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err)
	}
	// Grab a dynamic interface that we can create informers from
	dynamicset, err := dynamic.NewForConfig(config)
	if err != nil {
		fmt.Printf("could not generate dynamic client for config")
	}

	// Create a factory object that we can say "hey, I need to watch this resource"
	// and it will give us back an informer for it
	informerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynamicset, 0, "test", nil)

	// Retrieve a "GroupVersionResource" type that we need when generating our informer from our dynamic factory
	gvr1, _ := schema.ParseResourceArg("pods.v1.")
	gvr2, _ := schema.ParseResourceArg("services.v1.")

	// Finally, create our informers!
	informer1 := informerFactory.ForResource(*gvr1)
	informer2 := informerFactory.ForResource(*gvr2)

	stopCh := make(chan struct{})
	go startWatching(stopCh, informer1.Informer())
	go startWatching(stopCh, informer2.Informer())

	sigCh := make(chan os.Signal, 0)
	signal.Notify(sigCh, os.Kill, os.Interrupt)

	<-sigCh
	close(stopCh)

	//kubeInformerFactory := kubeinformers.NewSharedInformerFactory(clientset, time.Second*30)
	//svcInformer := kubeInformerFactory.Core().V1().Services().Informer()
	//
	//svcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
	//	AddFunc: func(obj interface{}) {
	//		fmt.Printf("service added: %s \n", obj)
	//	},
	//	DeleteFunc: func(obj interface{}) {
	//		fmt.Printf("service deleted: %s \n", obj)
	//	},
	//},)
	//
	//podInformer := kubeInformerFactory.Core().V1().Pods().Informer()
	//
	//podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
	//	AddFunc: func(obj interface{}) {
	//		fmt.Printf("pod added: %s \n", obj)
	//	},
	//	DeleteFunc: func(obj interface{}) {
	//		fmt.Printf("pod deleted: %s \n", obj)
	//	},
	//},)
	//
	//stop := make(chan struct{})
	//defer close(stop)
	//kubeInformerFactory.Start(stop)
	//for {
	//	time.Sleep(time.Second)
	//}
}

func startWatching(stopCh <-chan struct{}, s cache.SharedIndexInformer) {
	handlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			jsonString, err := json.Marshal(u)
			fmt.Println(err)
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