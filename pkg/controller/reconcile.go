package controller

import (
	"context"

	platformv1alpha1 "github.com/sri2103/tenant-operator/pkg/apis/platform/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (c *Controller) reconcileTenant(ctx context.Context, tenant *platformv1alpha1.Tenant) error {
	ns := tenant.Spec.NamespaceName
	klog.Infof("Reconciling Tenant %s (namespace: %s)", tenant.Name, ns)

	// 1️⃣ Ensure Namespace
	if err := c.ensureNamespace(ctx, tenant); err != nil {
		return err
	}

	// 2️⃣ Apply ResourceQuota
	if tenant.Spec.ResourceQuota != nil {
		if err := c.ensureResourceQuota(ctx, tenant); err != nil {
			return err
		}
	}

	// 3️⃣ Apply LimitRange
	if tenant.Spec.LimitRange != nil {
		if err := c.ensureLimitRange(ctx, tenant); err != nil {
			return err
		}
	}

	// 4️⃣ Apply NetworkPolicy
	if tenant.Spec.NetworkPolicy != nil {
		if err := c.ensureNetworkPolicy(ctx, tenant); err != nil {
			return err
		}
	}

	// 5️⃣ Apply RBAC
	if tenant.Spec.RBAC != nil {
		if err := c.ensureRBAC(ctx, tenant); err != nil {
			return err
		}
	}

	// 6️⃣ Update Tenant status
	tenant.Status.Phase = "Ready"
	tenant.Status.ResourcesCreated = []string{"Namespace", "Quota", "RBAC"}
	_, err := c.platformClient.PlatformV1alpha1().Tenants(tenant.Namespace).UpdateStatus(ctx, tenant, metav1.UpdateOptions{})
	if err != nil {
		klog.InfoS("error updating", "err", err.Error())
		return err
	}
	return nil
}
