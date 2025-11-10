package controller

import (
	"context"
	"fmt"

	"github.com/sri2103/tenant-operator/pkg/apis/platform/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (c *Controller) handleDeletion(ctx context.Context, tenant *v1alpha1.Tenant) error {
	nsName := tenant.Spec.NamespaceName
	if nsName == "" {
		nsName = fmt.Sprintf("tenant-%s", tenant.Name)
	}

	// Delete namespace (best-effort)
	if err := c.kubeClient.CoreV1().Namespaces().Delete(ctx, nsName, metav1.DeleteOptions{}); err != nil {
		klog.InfoS("error for deleting the spec namespace", "err", err)
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete namespace of specs %s:%w", nsName, err)
		}
	}
	klog.InfoS("Triggered namespace deletion for tenant", "namespace", nsName, "tenant", tenant.Name)
	// addition resources cleanup(DNS,external resources) can be added here.
	return nil
}
