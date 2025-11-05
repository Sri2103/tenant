package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sri2103/tenant-operator/pkg/apis/platform/v1alpha1"
	"github.com/sri2103/tenant-operator/pkg/controller"
	"github.com/sri2103/tenant-operator/pkg/generated/clientset/versioned"
	"github.com/sri2103/tenant-operator/pkg/generated/informers/externalversions"
	"github.com/sri2103/tenant-operator/pkg/setup"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
)

func main() {
	config, err := setup.PrepareConfig()
	// returned rest config
	if err != nil {
		log.Fatalf("error loading config: %v", err)
	}

	// core client
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error build core client: %v", err.Error())
	}

	// generate clientSet
	customClient, err := versioned.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error building custom client: %v", err.Error())
	}

	coreFactory := informers.NewSharedInformerFactory(clientSet, 30*time.Second)

	customFactory := externalversions.NewSharedInformerFactory(customClient, 30*time.Second)

	// end signals
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)

	// stop channel to block
	stopCh := make(chan struct{})

	go coreFactory.Start(stopCh)
	go customFactory.Start(stopCh)

	{
		scheme := runtime.NewScheme()
		v1alpha1.Install(scheme)

		rec := record.NewBroadcaster()
		rec.StartRecordingToSink(&v1.EventSinkImpl{
			Interface: clientSet.CoreV1().Events(""),
		})

		recorder := rec.NewRecorder(scheme, corev1.EventSource{Component: "tenant-controller"})
		ctr := controller.New(clientSet, customClient, recorder)
		go ctr.Run(stopCh, 10)
	}

	log.Println("Tenant controller started 🚀")
	<-sigterm
	close(stopCh)
}
