package controller

import (
	"context"
	"fmt"

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
	Phase := "Ready"
	ResourcesCreated := []string{
		fmt.Sprintf("Namespace/%s", ns),
		fmt.Sprintf("ResourceQuota/%s", ns),
		fmt.Sprintf("LimitRange/%s", ns),
		fmt.Sprintf("NetworkPolicy/%s", ns),
		fmt.Sprintf("RBAC/%s", ns),
	}
	Message := "Reconciled"
	//_, err := c.platformClient.PlatformV1alpha1().Tenants(tenant.Namespace).UpdateStatus(ctx, tenant, metav1.UpdateOptions{})

	err := c.updateStatus(ctx, tenant, Phase, ResourcesCreated, Message)
	if err != nil {
		klog.InfoS("error updating", "err", err.Error())
		return err
	}

	return nil
}

func (c *Controller) updateStatus(
	ctx context.Context,
	tenant *platformv1alpha1.Tenant,
	phase string,
	resources []string,
	reason string,
) error {
	tn := tenant.DeepCopy()
	tn.Status.Phase = phase
	tn.Status.ResourcesCreated = resources

	cond := platformv1alpha1.TenantCondition{
		Type:               "Reconciled",
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		LastTransitionTime: metav1.Now(),
	}
	tn.Status.Conditions = append(tn.Status.Conditions, cond)

	if _, err := c.platformClient.PlatformV1alpha1().Tenants(tn.Namespace).UpdateStatus(ctx, tn, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to list tenents during sync")
	}
	klog.InfoS("successfully reconciled tenant", "tenant", tn.Name, "namespace", tn.Namespace)
	return nil
}
