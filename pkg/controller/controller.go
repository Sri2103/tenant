package controller

import (
	"context"

	"github.com/sri2103/tenant-operator/pkg/generated/clientset/versioned"
	"github.com/sri2103/tenant-operator/pkg/generated/informers/externalversions"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

type controller struct {
	coreClient              kubernetes.Interface
	customClient            versioned.Interface
	recorder                record.EventRecorder
	inbuiltFactory          informers.SharedInformerFactory
	versioneInformerFactory externalversions.SharedInformerFactory
	queue                   workqueue.TypedRateLimitingInterface[any]
}

func New(coreClient kubernetes.Interface, customClient versioned.Interface, recorder record.EventRecorder) *controller {
	q := workqueue.
		NewNamedRateLimitingQueue(
			workqueue.DefaultTypedItemBasedRateLimiter[any](),
			"tenant-resource-enforcer",
		)
	return &controller{
		coreClient:   coreClient,
		customClient: customClient,
		recorder:     recorder,
		queue:        q,
	}
}

// main core runner for controller
func (c *controller) Run(stop <-chan struct{}, workers int) {
	<-stop
}

// worker setup to add to workerqueue
func (c *controller) enqueueAction(obj interface{}) {
}

// process from queue
func (c *controller) processNextItem() bool {
	return false
}

// sync Handler for complete controller
func (c *controller) syncResource(ctx context.Context) error {
	return nil
}

// update subresource for schema of crd
func (c *controller) updateSubresource(ctx context.Context) error {
	return nil
}
