package controller

import (
	"fmt"
	"time"

	clientset "github.com/sri2103/tenant-operator/pkg/generated/clientset/versioned"
	informers "github.com/sri2103/tenant-operator/pkg/generated/informers/externalversions/platform/v1alpha1"
	listers "github.com/sri2103/tenant-operator/pkg/generated/listers/platform/v1alpha1"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	ControllerName  = "tenant-controller"
	tenantFinalizer = "platform.myorg.io/tenant-cleanup"
	defaultResync   = 10 * time.Minute
)

type Controller struct {
	kubeClient     kubernetes.Interface
	platformClient clientset.Interface
	tenantLister   listers.TenantLister
	tenantSynced   cache.InformerSynced
	queue          workqueue.TypedRateLimitingInterface[any]
}

func NewController(
	kubeClient kubernetes.Interface,
	platformClient clientset.Interface,
	tenantInformer informers.TenantInformer,
) *Controller {
	c := &Controller{
		kubeClient:     kubeClient,
		platformClient: platformClient,
		tenantLister:   tenantInformer.Lister(),
		tenantSynced:   tenantInformer.Informer().HasSynced,
		queue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[any](), "tenant"),
	}

	klog.Info("Setting up event handlers for Tenant controller")

	tenantInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.enqueue,
		UpdateFunc: func(oldObj, newObj interface{}) { c.enqueue(newObj) },
		DeleteFunc: c.enqueue,
	})

	return c
}

func (c *Controller) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.queue.Add(key)
}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.InfoS("Starting Tenant controller", "worker", threadiness)
	klog.Info("waiting for informer cache to sync")

	if ok := cache.WaitForCacheSync(stopCh, c.tenantSynced); !ok {
		klog.Error("failed to wait for caches to sync")
		return
	}

	klog.Info("caches synced, starting workers")

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	go wait.Until(c.syncPeriodic, defaultResync, stopCh)

	klog.Info("Tenant controller is running")
	<-stopCh
	klog.Info("Shutting down Tenant controller")
}

func (c *Controller) runWorker() {
	
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {

	obj, shutdown := c.queue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.queue.Done(obj)
		key, ok := obj.(string)
		if !ok {
			c.queue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in queue but got %#v", obj))
			return nil
		}

		if err := c.syncHandler(key); err != nil {
			klog.InfoS("sync error:", "err", err)
			c.queue.AddRateLimited(key)
			return fmt.Errorf("error syncing Tenant %q: %v", key, err)
		}

		c.queue.Forget(obj)
		// klog.Infof("Successfully synced Tenant %q", key)
		return nil
	}(obj)
	if err != nil {
		klog.InfoS("error after trying to sync", "err", err)
		utilruntime.HandleError(err)
	}

	return true
}
