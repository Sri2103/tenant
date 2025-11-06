package controller

import (
	"context"

	platformv1alpha1 "github.com/sri2103/tenant-operator/pkg/apis/platform/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
	_, err := c.platformClient.PlatformV1alpha1().Tenants().UpdateStatus(ctx, tenant, metav1.UpdateOptions{})
	return err
}

func (c *Controller) ensureNamespace(ctx context.Context, t *platformv1alpha1.Tenant) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: t.Spec.NamespaceName}}
	_, err := c.kubeClient.CoreV1().Namespaces().Get(ctx, ns.Name, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	_, err = c.kubeClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	return err
}

func (c *Controller) ensureResourceQuota(ctx context.Context, t *platformv1alpha1.Tenant) error {
	rq := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-quota",
			Namespace: t.Spec.NamespaceName,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				corev1.ResourceCPU:    resourceQuantity(t.Spec.ResourceQuota.CPU),
				corev1.ResourceMemory: resourceQuantity(t.Spec.ResourceQuota.Memory),
				corev1.ResourcePods:   *resource.NewQuantity(int64(t.Spec.ResourceQuota.Pods), resource.DecimalSI),
			},
		},
	}
	_, err := c.kubeClient.CoreV1().ResourceQuotas(t.Spec.NamespaceName).Create(ctx, rq, metav1.CreateOptions{})
	return ignoreAlreadyExists(err)
}

func (c *Controller) ensureLimitRange(ctx context.Context, t *platformv1alpha1.Tenant) error {
	// similar pattern — create if not exist
	return nil
}

func (c *Controller) ensureNetworkPolicy(ctx context.Context, t *platformv1alpha1.Tenant) error {
	// create basic deny-all or custom allow policy
	return nil
}

func (c *Controller) ensureRBAC(ctx context.Context, t *platformv1alpha1.Tenant) error {
	// create RoleBindings for AdminUsers and ViewerUsers
	return nil
}
