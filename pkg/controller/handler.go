package controller

import (
	"context"

	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (c *Controller) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	tenant, err := c.tenantLister.Tenants(namespace).Get(name)
	if err != nil {
		return err
	}

	klog.Info(tenant, "fetched request item")

	return c.reconcileTenant(context.Background(), tenant.DeepCopy())
}
