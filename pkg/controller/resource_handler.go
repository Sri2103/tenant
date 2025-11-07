package controller

import (
	"context"

	platformv1alpha1 "github.com/sri2103/tenant-operator/pkg/apis/platform/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
