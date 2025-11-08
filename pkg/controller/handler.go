package controller

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (c *Controller) syncHandler(key string) error {
	ctx := context.Background()
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	tenant, err := c.tenantLister.Tenants(namespace).Get(name)
	if err != nil {
		return err
	}

	tenantCopy := tenant.DeepCopy()

	if tenantCopy.DeletionTimestamp != nil {
		if containsString(tenantCopy.Finalizers, tenantFinalizer) {
			if err := c.handleDeletion(ctx, tenantCopy); err != nil {
				return err
			}

			tenantCopy.Finalizers = removeString(tenantCopy.Finalizers, tenantFinalizer)

			if _, err := c.
				platformClient.
				PlatformV1alpha1().
				Tenants(tenantCopy.Namespace).
				Update(ctx, tenantCopy, metav1.UpdateOptions{}); err != nil {
				return fmt.Errorf("failed to remove finalizer:%w", err)
			}
		}
		return nil
	}

	// ensure the presence of finalizer
	if !containsString(tenantCopy.Finalizers, tenantFinalizer) {
		tenantCopy.Finalizers = append(tenantCopy.Finalizers, tenantFinalizer)
		if _, err := c.platformClient.
			PlatformV1alpha1().
			Tenants(tenantCopy.Namespace).
			Update(ctx, tenantCopy, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to add finalizer:%w", err)
		}
		return nil
	}

	err = c.reconcileTenant(context.Background(), tenantCopy)
	if err != nil {
		return err
	}

	return nil
}

func (c *Controller) syncPeriodic() {
	klog.Info("periodic resync: enqueing all tenants")

	tenants, err := c.tenantLister.List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	for _, t := range tenants {
		c.enqueue(t)
	}
}
