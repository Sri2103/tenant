package controller

import (
	"context"
	"fmt"

	platformv1alpha1 "github.com/sri2103/tenant-operator/pkg/apis/platform/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (c *Controller) ensureNamespace(ctx context.Context, tenant *platformv1alpha1.Tenant) error {
	_, err := c.kubeClient.CoreV1().Namespaces().Get(ctx, tenant.Spec.NamespaceName, metav1.GetOptions{})
	if err == nil {
		return nil
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: tenant.Spec.NamespaceName,
			Labels: map[string]string{
				"tenant":     tenant.Name,
				"managed-by": "tenant-controller",
			},
			Annotations: map[string]string{
				"tenant.platform.myorg.io/owner": tenant.Spec.Owner,
			},
		},
	}

	_, err = c.kubeClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}

	klog.InfoS("created namespace for tenant", "namespace", tenant.Spec.NamespaceName, "tenant", tenant.Name)
	return nil
}

func (c *Controller) ensureResourceQuota(ctx context.Context, tenant *platformv1alpha1.Tenant) error {
	spec := tenant.Spec.ResourceQuota

	if spec == nil {
		klog.InfoS("resource quota not present", "tenant", tenant.Name)
		return nil
	}

	rqName := "tenant-quota"
	ns := tenant.Spec.NamespaceName

	_, err := c.kubeClient.CoreV1().ResourceQuotas(ns).Get(ctx, rqName, metav1.GetOptions{})

	if err == nil {
		return nil
	}

	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get resourcequota:%w", err)
	}

	rq := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-quota",
			Namespace: ns,
			Labels:    map[string]string{"tenant": tenant.Name},
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{},
		},
	}
	if spec.CPU != "" {
		rq.Spec.Hard[corev1.ResourceCPU] = resource.MustParse(spec.CPU)
	}

	if spec.Memory != "" {
		rq.Spec.Hard[corev1.ResourceMemory] = resource.MustParse(spec.Memory)
	}

	if spec.Pods > 0 {
		rq.Spec.Hard[corev1.ResourcePods] = *resource.NewQuantity(int64(spec.Pods), resource.DecimalSI)
	}

	_, err = c.kubeClient.CoreV1().ResourceQuotas(ns).Create(ctx, rq, metav1.CreateOptions{})
	return ignoreAlreadyExists(err)
}

func (c *Controller) ensureLimitRange(ctx context.Context, tenant *platformv1alpha1.Tenant) error {
	spec := tenant.Spec.LimitRange
	ns := tenant.Spec.NamespaceName
	if spec == nil {
		return nil
	}

	lrName := "tenant-limits"

	_, err := c.kubeClient.CoreV1().LimitRanges(ns).Get(ctx, lrName, metav1.GetOptions{})

	if err == nil {
		return nil
	}

	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to fetch the limitRange: %w", err)
	}

	lr := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      lrName,
			Namespace: ns,
			Labels:    map[string]string{"tenant": tenant.Name},
		},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				{
					Type:           corev1.LimitTypeContainer,
					DefaultRequest: corev1.ResourceList{},
					Default:        corev1.ResourceList{},
				},
			},
		},
	}

	if spec.DefaultRequest != nil {
		if spec.DefaultRequest.CPU != "" {
			lr.Spec.Limits[0].DefaultRequest[corev1.ResourceCPU] = resource.MustParse(spec.DefaultRequest.CPU)
		}

		if spec.DefaultRequest.Memory != "" {
			lr.Spec.Limits[0].DefaultRequest[corev1.ResourceMemory] = resource.MustParse(spec.DefaultRequest.Memory)
		}
	}

	if spec.DefaultLimit != nil {
		if spec.DefaultLimit.CPU != "" {
			lr.Spec.Limits[0].Default[corev1.ResourceCPU] = resource.MustParse(spec.DefaultLimit.CPU)
		}

		if spec.DefaultLimit.Memory != "" {
			lr.Spec.Limits[0].Default[corev1.ResourceMemory] = resource.MustParse(spec.DefaultLimit.Memory)
		}
	}

	if _, err := c.kubeClient.CoreV1().LimitRanges(ns).Create(ctx, lr, metav1.CreateOptions{}); err != nil {
		return nil
	}

	klog.InfoS("Created LimitRange", "namespace", ns, "tenant", tenant.Name)
	return nil
}

func (c *Controller) ensureNetworkPolicy(ctx context.Context, tenant *platformv1alpha1.Tenant) error {
	spec := tenant.Spec.NetworkPolicy

	if spec == nil {
		// no spec return
		return nil
	}

	npName := "tenant-network"
	ns := tenant.Spec.NamespaceName

	_, err := c.kubeClient.NetworkingV1().NetworkPolicies(ns).Get(ctx, npName, metav1.GetOptions{})

	if err == nil {
		return nil
	}

	if !errors.IsNotFound(err) {
		return fmt.Errorf("error fetching the network policy: %w", err)
	}

	np := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      npName,
			Namespace: ns,
			Labels:    map[string]string{"tenant": tenant.Name},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress, networkingv1.PolicyTypeIngress,
			},
		},
	}

	if len(spec.AllowNamespaces) > 0 {
		var peers []networkingv1.NetworkPolicyPeer

		for _, ns := range spec.AllowNamespaces {
			peers = append(peers, networkingv1.NetworkPolicyPeer{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"tenant-namespace": ns},
				},
			})
		}
		np.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{{From: peers}}
	}

	//  if deny external is true, set egress to namespace-only (no ip blocks)
	if spec.DenyExternal {
		np.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{
			{
				To: []networkingv1.NetworkPolicyPeer{{NamespaceSelector: &metav1.LabelSelector{}}},
			},
		}
	}

	if _, err := c.kubeClient.NetworkingV1().NetworkPolicies(ns).Create(ctx, np, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create networking policies: %w", err)
	}

	klog.InfoS("Created Network Policy", "namespace", ns, "tenant", tenant.Name)
	return nil
}

func (c *Controller) ensureRBAC(ctx context.Context, tenant *platformv1alpha1.Tenant) error {
	spec := tenant.Spec.RBAC
	nsName := tenant.Spec.NamespaceName

	roleName := "tenant-admin"
	if _, err := c.kubeClient.RbacV1().Roles(nsName).Get(ctx, roleName, metav1.GetOptions{}); errors.IsNotFound(err) {
		role := &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleName,
				Namespace: nsName,
				Labels:    map[string]string{"tenant": tenant.Name},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"", "apps", "batch"},
					Resources: []string{"pods", "deployments", "replicasets", "services", "configmaps"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
			},
		}
		if _, err := c.kubeClient.RbacV1().Roles(nsName).Create(ctx, role, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("failed to create role:%w", err)
		}
	}

	// create rolebinding for admin users
	for i, u := range spec.AdminUsers {
		bindingName := fmt.Sprintf("tenant-admin-%d", i)
		if _, err := c.kubeClient.RbacV1().RoleBindings(nsName).Get(ctx, bindingName, metav1.GetOptions{}); errors.IsNotFound(err) {
			rb := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName,
					Namespace: nsName,
					Labels:    map[string]string{"tenant": tenant.Name},
				},
				Subjects: []rbacv1.Subject{{
					Kind: "User", Name: u,
				}},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.SchemeGroupVersion.Group,
					Kind:     "Role",
					Name:     roleName,
				},
			}

			if _, err := c.kubeClient.RbacV1().RoleBindings(nsName).Create(ctx, rb, metav1.CreateOptions{}); err != nil {
				return fmt.Errorf("failed to create rolebinding:%w", err)
			}

		} else if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get rolbebinding: %w", err)
		}
	}

	return nil
}
