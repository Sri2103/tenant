package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/sri2103/tenant-operator/pkg/apis/platform/v1alpha1"
	clientset "github.com/sri2103/tenant-operator/pkg/generated/clientset/versioned"
	tenantInformer "github.com/sri2103/tenant-operator/pkg/generated/informers/externalversions/platform/v1alpha1"
	tenantListers "github.com/sri2103/tenant-operator/pkg/generated/listers/platform/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreInformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	coreListers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	ControllerName     = "tenant-controller"
	tenantFinalizer    = "platform.myorg.io/tenant-cleanup"
	defaultWorkerCount = 2
	defaultResync      = 10 * time.Minute
)

type GenereatedController struct {
	kubeclient     kubernetes.Interface
	tenantClient   clientset.Interface
	tenantInformer tenantInformer.TenantInformer
	tenantLister   tenantListers.TenantLister
	nsInformer     coreInformers.NamespaceInformer
	nsLister       coreListers.NamespaceLister
	queue          workqueue.TypedRateLimitingInterface[any]
}

func NewController(
	kubeclient kubernetes.Interface,
	tenantClient clientset.Interface,
	tenantInformer tenantInformer.TenantInformer,
	nsInformer coreInformers.NamespaceInformer,
) *GenereatedController {
	g := &GenereatedController{
		kubeclient:     kubeclient,
		tenantClient:   tenantClient,
		tenantInformer: tenantInformer,
		nsInformer:     nsInformer,
		tenantLister:   tenantInformer.Lister(),
		nsLister:       nsInformer.Lister(),
		queue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[any](), "tenants"),
	}

	tenantInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { g.enqueueGenerated(obj) },
		UpdateFunc: func(oldObj, newObj interface{}) { g.enqueueGenerated(newObj) },
		DeleteFunc: func(obj interface{}) { g.enqueueGenerated(obj) },
	})

	return g
}

func (c *GenereatedController) enqueueGenerated(obj interface{}) {
	key, err := cache.MetaNamespaceIndexFunc(obj)
	if err != nil {
		runtime.HandleError(fmt.Errorf("failed to getkey from object:%w", err))
		return
	}

	c.queue.Add(key)
}

func (c *GenereatedController) RunGenerated(ctx context.Context, workers int) error {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	if workers <= 0 {
		workers = defaultWorkerCount
	}

	klog.InfoS("starting Tenant controller", "workers", workers)
	klog.Info("waiting for informer cache to sync")

	// waiting for informer cache to sync
	if ok := cache.WaitForCacheSync(ctx.Done(), c.tenantInformer.Informer().HasSynced, c.nsInformer.Informer().HasSynced); !ok {
		return fmt.Errorf("failed to load cached sync")
	}

	klog.Info("caches synced, starting workers")

	// start worker go routine
	for i := 0; i < workers; i++ {
		// this manages backing off also
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	// start periodic sync
	go wait.UntilWithContext(ctx, c.resyncAll, defaultResync)

	<-ctx.Done()

	klog.Info("shutting down the controller")

	return nil
}

func (c *GenereatedController) runWorker(ctx context.Context) {
	for c.processNextItem(ctx) {
	}
}

func (c *GenereatedController) processNextItem(ctx context.Context) bool {
	item, shutDown := c.queue.Get()
	if shutDown {
		return false
	}

	key, ok := item.(string)
	if !ok {
		c.queue.Forget(item)
		runtime.HandleError(fmt.Errorf("expected string key, but got:%v", item))
		return true
	}

	if err := c.syncHandler(ctx, key); err != nil {
		runtime.HandleError(fmt.Errorf("error synching tenant %q:%w", key, err))
		c.queue.AddRateLimited(key)
		return true
	}

	// succesfully handled the item from queue
	c.queue.Forget(item)
	return true
}

// sync Handler
// order of setting
// 1. fetch tenant object
// 2. Handle deletion and finalizer
// 3. Ensure finalizer present
// 4. Ensure Namespace
// 5.
func (c *GenereatedController) syncHandler(ctx context.Context, key string) error {
	klog.InfoS("reconciling tenant", "key", key)
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("invalid resource key: %w", err)
	}

	// 1.fetch tenant
	tenant, err := c.tenantLister.Tenants(ns).Get(name)

	if errors.IsNotFound(err) {
		klog.InfoS("no resource found, nothing to do", "key", key)
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to get tenant %s:%w", key, err)
	}

	tenantCopy := tenant.DeepCopy()

	// 2. deletion check: if deletion timestamp set run finalizer logic
	if tenantCopy.DeletionTimestamp != nil {
		if containsString(tenantCopy.Finalizers, tenantFinalizer) {
			if err := c.handleDeletion(ctx, tenantCopy); err != nil {
				return err
			}

			tenantCopy.Finalizers = removeString(tenantCopy.Finalizers, tenantFinalizer)

			if _, err := c.
				tenantClient.
				PlatformV1alpha1().
				Tenants(tenantCopy.Namespace).
				Update(ctx, tenantCopy, metav1.UpdateOptions{}); err != nil {
				return fmt.Errorf("failed to remove finalizer:%w", err)
			}
		}
		return nil
	}

	// 3. Ensure the presence of finalizer
	if !containsString(tenantCopy.Finalizers, tenantFinalizer) {
		tenantCopy.Finalizers = append(tenantCopy.Finalizers, tenantFinalizer)
		if _, err := c.tenantClient.
			PlatformV1alpha1().
			Tenants(tenantCopy.Namespace).
			Update(ctx, tenantCopy, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to add finalizer:%w", err)
		}
		return nil
	}

	created := []string{}

	// 4. Ensure Namespace
	nsName := tenantCopy.Spec.NamespaceName

	if nsName == "" {
		nsName = fmt.Sprintf("tenant-%s", tenantCopy.Name)
	}
	if err := c.ensureNamespace(ctx, tenantCopy, nsName); err != nil {
		return err
	}

	created = append(created, fmt.Sprintf("Namespace/%s", nsName))

	// 5.Ensure ResourceQuota
	if tenantCopy.Spec.ResourceQuota != nil {
		if err := c.ensureResourceQuota(ctx, tenantCopy, nsName); err != nil {
			return err
		}
		created = append(created, fmt.Sprintf("ResourceQuota/%s", nsName))
	}

	// 6.Ensure LimitRange
	if tenantCopy.Spec.LimitRange != nil {
		if err := c.ensureLimitRange(ctx, tenantCopy, nsName); err != nil {
			return err
		}
		created = append(created, fmt.Sprintf("LimitRange/%s", nsName))
	}

	// 7.Ensure NetworkPolicy
	if tenantCopy.Spec.NetworkPolicy != nil {
		if err := c.ensureNetworkPolicy(ctx, tenantCopy, nsName); err != nil {
			return err
		}
		created = append(created, fmt.Sprintf("NetworkPolicy/%s", nsName))
	}

	// 8. Ensure RBAC
	if tenantCopy.Spec.RBAC != nil {
		if err := c.ensureRBAC(ctx, tenantCopy, nsName); err != nil {
			return err
		}
		created = append(created, fmt.Sprintf("RBAC/%s", nsName))
	}

	// 9. update status to Ready
	if err := c.updateStatus(ctx, tenantCopy, "Ready", created, "Reconciled"); err != nil {
		return err
	}

	klog.InfoS("successfully reconciled tenant", "tenant", tenantCopy.Name, "namespace", tenantCopy.Namespace)
	return nil
}

// handle deletion
func (c *GenereatedController) handleDeletion(ctx context.Context, tenant *v1alpha1.Tenant) error {
	nsName := tenant.Spec.NamespaceName
	if nsName == "" {
		nsName = fmt.Sprintf("tenant-%s", tenant.Name)
	}

	// Delete namespace (best-effort)
	if err := c.kubeclient.CoreV1().Namespaces().Delete(ctx, nsName, metav1.DeleteOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete namespace %s:%w", nsName, err)
		}
	}
	klog.InfoS("Triggered namespace deletion for tenant", "namespace", nsName, "tenant", tenant.Name)
	// addition resources cleanup(DNS,external resources) can be added here.
	return nil
}

// Ensure Helpers (Namespace -> ResourceQuota -> LimitRange -> Networkpolicy -> RRBAC)
func (c *GenereatedController) ensureNamespace(ctx context.Context, tenant *v1alpha1.Tenant, nsName string) error {
	_, err := c.nsLister.Get(nsName)
	if err == nil {
		// namespace exists
		return nil
	}

	if !errors.IsNotFound(err) {
		return fmt.Errorf("error checking namespace: %s:%w", nsName, err)
	}

	// Create NameSpace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
			Labels: map[string]string{
				"tenant":     tenant.Name,
				"managed-by": "tenant-controller",
			},
			Annotations: map[string]string{
				"tenant.platform.myorg.io/owner": tenant.Spec.Owner,
			},
		},
	}

	if _, err := c.kubeclient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create namespace %s:%w", nsName, err)
	}

	klog.InfoS("created namespace for tenant", "namespace", nsName, "tenant", tenant.Name)
	return nil
}

// ensureResourceQuota
func (c *GenereatedController) ensureResourceQuota(ctx context.Context, tenant *v1alpha1.Tenant, nsName string) error {
	spec := tenant.Spec.ResourceQuota

	if spec == nil {
		return nil
	}

	rqName := "tenant-quota"

	_, err := c.kubeclient.CoreV1().ResourceQuotas(nsName).Get(ctx, rqName, metav1.GetOptions{})
	if err == nil {
		return nil
	}

	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get resourcequota:%w", err)
	}

	rq := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rqName,
			Namespace: nsName,
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

	if _, err := c.kubeclient.CoreV1().ResourceQuotas(nsName).Create(ctx, rq, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create resourcequota: %w", err)
	}

	klog.InfoS("Created resource quotas", "namespace", nsName, "tenant", tenant.Name)
	return nil
}

// ensureLimitRange
func (c *GenereatedController) ensureLimitRange(ctx context.Context, tenant *v1alpha1.Tenant, nsName string) error {
	spec := tenant.Spec.LimitRange
	if spec == nil {
		return nil
	}

	lrName := "tenant-limits"

	_, err := c.kubeclient.CoreV1().LimitRanges(nsName).Get(ctx, lrName, metav1.GetOptions{})
	if err == nil {
		return nil
	}

	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get limitRange %w", err)
	}

	lr := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      lrName,
			Namespace: nsName,
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

	if _, err := c.kubeclient.CoreV1().LimitRanges(nsName).Create(ctx, lr, metav1.CreateOptions{}); err != nil {
		return nil
	}

	klog.InfoS("Created LimitRange", "namespace", nsName, "tenant", tenant.Name)
	return nil
}

// ensureNetworkPolicy
func (c *GenereatedController) ensureNetworkPolicy(ctx context.Context, tenant *v1alpha1.Tenant, nsName string) error {
	spec := tenant.Spec.NetworkPolicy

	if spec == nil {
		// no spec then return
		return nil
	}

	npName := "tenant-network"

	_, err := c.kubeclient.NetworkingV1().NetworkPolicies(nsName).Get(ctx, npName, metav1.GetOptions{})
	if err == nil {
		return nil
	}

	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get networkpolicy %w", err)
	}

	np := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      npName,
			Namespace: nsName,
			Labels:    map[string]string{"tenant": tenant.Name},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress, networkingv1.PolicyTypeIngress},
		},
	}

	// build ingress peers for allowed namespaces(requires those namespaces to be labelled appropriately)
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

	if _, err := c.kubeclient.NetworkingV1().NetworkPolicies(nsName).Create(ctx, np, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create networking policies: %w", err)
	}

	klog.InfoS("Created Network Policy", "namespace", nsName, "tenant", tenant.Name)
	return nil
}

// ensure RBAC
func (c *GenereatedController) ensureRBAC(ctx context.Context, tenant *v1alpha1.Tenant, nsName string) error {
	spec := tenant.Spec.RBAC

	if spec == nil {
		return nil
	}

	roleName := "tenant-admin"
	if _, err := c.kubeclient.RbacV1().Roles(nsName).Get(ctx, roleName, metav1.GetOptions{}); errors.IsNotFound(err) {
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
		if _, err := c.kubeclient.RbacV1().Roles(nsName).Create(ctx, role, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("failed to create role:%w", err)
		}
	}

	// create rolebinding for admin users
	for i, u := range spec.AdminUsers {
		bindingName := fmt.Sprintf("tenant-admin-%d", i)
		if _, err := c.kubeclient.RbacV1().RoleBindings(nsName).Get(ctx, bindingName, metav1.GetOptions{}); errors.IsNotFound(err) {
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

			if _, err := c.kubeclient.RbacV1().RoleBindings(nsName).Create(ctx, rb, metav1.CreateOptions{}); err != nil {
				return fmt.Errorf("failed to create rolebinding:%w", err)
			}

		} else if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get rolbebinding: %w", err)
		}
	}

	return nil
}

// save status
func (c *GenereatedController) updateStatus(
	ctx context.Context,
	tenant *v1alpha1.Tenant,
	phase string,
	resources []string,
	reason string,
) error {
	tn := tenant.DeepCopy()
	tn.Status.Phase = phase
	tn.Status.ResourcesCreated = resources

	cond := v1alpha1.TenantCondition{
		Type:               "Reconciled",
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		LastTransitionTime: metav1.Now(),
	}
	tn.Status.Conditions = append(tn.Status.Conditions, cond)

	if _, err := c.tenantClient.PlatformV1alpha1().Tenants(tn.Namespace).UpdateStatus(ctx, tn, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to list tenents during sync")
	}
	return nil
}

// resyncAll enqueues all tenants periodically for reconciliation
func (c *GenereatedController) resyncAll(ctx context.Context) {
	klog.Info("periodic resync: enqueing all tenants")
	tenants, err := c.tenantLister.List(labels.Everything())
	if err != nil {
		klog.ErrorS(err, "failed to list tenants during resync")
		return
	}
	for _, t := range tenants {
		c.enqueueGenerated(t)
	}
}

func containsString(slice []string, b string) bool {
	for _, v := range slice {
		if v == b {
			return true
		}
	}
	return false
}

func removeString(slice []string, b string) []string {
	out := []string{}
	for _, v := range slice {
		if v == b {
			continue
		}
		out = append(out, v)
	}
	return out
}
