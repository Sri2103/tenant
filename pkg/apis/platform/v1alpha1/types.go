
// Copyright 2025.
// Licensed under the Apache License, Version 2.0 (the "License");

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Tenant defines a multi-tenant environment in the Internal Developer Platform.
type Tenant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TenantSpec   `json:"spec,omitempty"`
	Status TenantStatus `json:"status,omitempty"`
}

// TenantSpec defines the desired state of a Tenant.
type TenantSpec struct {
	Owner         string             `json:"owner"`
	NamespaceName string             `json:"namespaceName"`
	ResourceQuota *ResourceQuotaSpec `json:"resourceQuota,omitempty"`
	LimitRange    *LimitRangeSpec    `json:"limitRange,omitempty"`
	NetworkPolicy *NetworkPolicySpec `json:"networkPolicy,omitempty"`
	RBAC          *RBACSpec          `json:"rbac,omitempty"`
	Policy        *PolicySpec        `json:"policy,omitempty"`
	Monitoring    *MonitoringSpec    `json:"monitoring,omitempty"`
}

// ResourceQuotaSpec defines resource limits for the tenant.
type ResourceQuotaSpec struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
	Pods   int32  `json:"pods,omitempty"`
}

// LimitRangeSpec defines default request and limit configurations.
type LimitRangeSpec struct {
	DefaultRequest *ResourceSpec `json:"defaultRequest,omitempty"`
	DefaultLimit   *ResourceSpec `json:"defaultLimit,omitempty"`
}

// ResourceSpec defines CPU and memory resource values.
type ResourceSpec struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// NetworkPolicySpec defines networking restrictions for a tenant.
type NetworkPolicySpec struct {
	AllowNamespaces []string `json:"allowNamespaces,omitempty"`
	DenyExternal    bool     `json:"denyExternal,omitempty"`
}

// RBACSpec defines administrative and viewer access for a tenant.
type RBACSpec struct {
	AdminUsers  []string `json:"adminUsers,omitempty"`
	ViewerUsers []string `json:"viewerUsers,omitempty"`
}

// PolicySpec defines container-level or registry enforcement policies.
type PolicySpec struct {
	EnforceRegistry string `json:"enforceRegistry,omitempty"`
	AllowPrivileged bool   `json:"allowPrivileged,omitempty"`
}

// MonitoringSpec defines monitoring configuration for a tenant.
type MonitoringSpec struct {
	Enable bool              `json:"enable,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

// TenantStatus defines the observed state of a Tenant.
type TenantStatus struct {
	Phase            string            `json:"phase,omitempty"`
	ResourcesCreated []string          `json:"resourcesCreated,omitempty"`
	Conditions       []TenantCondition `json:"conditions,omitempty"`
}

// TenantCondition represents the state of a tenant resource at a certain point.
type TenantCondition struct {
	Type               string                 `json:"type"`
	Status             metav1.ConditionStatus `json:"status"`
	Reason             string                 `json:"reason,omitempty"`
	LastTransitionTime metav1.Time            `json:"lastTransitionTime,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TenantList contains a list of Tenants.
type TenantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Tenant `json:"items"`
}
