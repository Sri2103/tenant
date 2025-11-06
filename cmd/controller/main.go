package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sri2103/tenant-operator/pkg/controller"
	clientset "github.com/sri2103/tenant-operator/pkg/generated/clientset/versioned"
	platformInformers "github.com/sri2103/tenant-operator/pkg/generated/informers/externalversions"
	"github.com/sri2103/tenant-operator/pkg/setup"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)



func main() {


	// ✅ Build Kubernetes and custom clients
	// cfg, err := buildConfig(masterURL, kubeconfig)
	cfg, err := setup.PrepareConfig()
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building core Kubernetes clientset: %v", err)
	}

	platformClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building platform clientset: %v", err)
	}

	// ✅ Shared informer factories
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, time.Minute*5)
	platformInformerFactory := platformInformers.NewSharedInformerFactory(platformClient, time.Minute*5)

	// ✅ Tenant informer from generated code
	tenantInformer := platformInformerFactory.Platform().V1alpha1().Tenants()

	// ✅ Create the controller
	controller := controller.NewController(kubeClient, platformClient, tenantInformer)

	// ✅ Signal handling
	stopCh := setupSignalHandler()

	// ✅ Start informers and controller
	klog.Info("Starting informer factories")
	kubeInformerFactory.Start(stopCh)
	platformInformerFactory.Start(stopCh)

	klog.Info("Starting Tenant controller workers")
	controller.Run(2, stopCh)
}



func setupSignalHandler() <-chan struct{} {
	stop := make(chan struct{})
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		close(stop)
	}()
	return stop
}
