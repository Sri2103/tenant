package controller

import (
	"context"

	"k8s.io/client-go/tools/cache"
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

	err = c.reconcileTenant(context.Background(), tenant.DeepCopy())
	if err != nil {
		return err
	}

	return nil
}
