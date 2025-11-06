package main

import (
	"context"
	"flag"
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
	"k8s.io/klog/v2"
)

func main() {
	threads := flag.Int("threads", 2, "Number of controller worker threads.")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	tenantClient, err := versioned.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error building custom client: %v", err.Error())
	}

	// setup informer factory
	coreFactory := informers.NewSharedInformerFactory(clientSet, 30*time.Second)
	tenantFactory := externalversions.NewSharedInformerFactory(tenantClient, 30*time.Second)

	// signal channel to stop the running factory
	stopCh := setupSignalHandler(cancel)
	go coreFactory.Start(stopCh)
	go tenantFactory.Start(stopCh)

	scheme := runtime.NewScheme()
	v1alpha1.Install(scheme)

	rec := record.NewBroadcaster()
	rec.StartRecordingToSink(&v1.EventSinkImpl{
		Interface: clientSet.CoreV1().Events(""),
	})

	recorder := rec.NewRecorder(scheme, corev1.EventSource{Component: "tenant-controller"})
	ctr := controller.New(
		clientSet,
		tenantClient,
		tenantFactory.Platform().V1alpha1().Tenants(),
		coreFactory.Core().V1().Namespaces(),
		recorder,
	)

	if err := ctr.Run(ctx, *threads); err != nil {
		klog.Fatalf("❌ Error running controller: %v", err)
	}

	log.Println("Tenant controller started 🚀")
}

func setupSignalHandler(cancel context.CancelFunc) <-chan struct{} {
	stop := make(chan struct{})
	go func() {
		c := make(chan os.Signal, 2)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		<-c
		klog.Info("🛑 Received termination signal. Shutting down gracefully...")
		cancel()
		close(stop)
	}()
	return stop
}
