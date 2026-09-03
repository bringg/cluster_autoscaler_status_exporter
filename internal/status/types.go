// Package status models the cluster-autoscaler-status ConfigMap document and
// turns it into Go values. It knows nothing about Kubernetes clients or
// Prometheus — it borrows apimachinery only for timestamp decoding.
package status

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Status is the whole cluster-autoscaler-status document.
type Status struct {
	AutoscalerStatus string      `json:"autoscalerStatus"`
	Message          string      `json:"message"`
	ClusterWide      ClusterWide `json:"clusterWide"`
	NodeGroups       []NodeGroup `json:"nodeGroups"`
	Time             string      `json:"time"`
}

// ClusterWide holds the conditions that apply to the whole cluster.
type ClusterWide struct {
	Health    ClusterHealth `json:"health"`
	ScaleUp   ScaleUp       `json:"scaleUp"`
	ScaleDown ScaleDown     `json:"scaleDown"`
}

// ClusterHealth is the cluster-wide health condition.
type ClusterHealth struct {
	Status             string      `json:"status"`
	NodeCounts         NodeCounts  `json:"nodeCounts"`
	LastProbeTime      metav1.Time `json:"lastProbeTime"`
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
}

// NodeGroup is the status of a single autoscaled node group.
type NodeGroup struct {
	Name      string          `json:"name"`
	Health    NodeGroupHealth `json:"health"`
	ScaleUp   ScaleUp         `json:"scaleUp"`
	ScaleDown ScaleDown       `json:"scaleDown"`
}

// NodeGroupHealth is a node group's health condition, including the bounds the
// autoscaler may scale it between.
type NodeGroupHealth struct {
	Status              string      `json:"status"`
	NodeCounts          NodeCounts  `json:"nodeCounts"`
	CloudProviderTarget int         `json:"cloudProviderTarget"`
	MinSize             int         `json:"minSize"`
	MaxSize             int         `json:"maxSize"`
	LastProbeTime       metav1.Time `json:"lastProbeTime"`
	LastTransitionTime  metav1.Time `json:"lastTransitionTime"`
}

// ScaleUp is a scale-up condition. BackoffInfo is populated for node groups only.
type ScaleUp struct {
	Status             string      `json:"status"`
	BackoffInfo        BackoffInfo `json:"backoffInfo"`
	LastProbeTime      metav1.Time `json:"lastProbeTime"`
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
}

// BackoffInfo describes why the autoscaler stopped attempting scale-ups.
type BackoffInfo struct {
	ErrorCode    string `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
}

// ScaleDown is a scale-down condition.
type ScaleDown struct {
	Status             string      `json:"status"`
	Candidates         int         `json:"candidates"`
	LastProbeTime      metav1.Time `json:"lastProbeTime"`
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
}

// NodeCounts breaks nodes down by how the autoscaler sees them.
type NodeCounts struct {
	Registered       RegisteredNodeCounts `json:"registered"`
	LongUnregistered int                  `json:"longUnregistered"`
	Unregistered     int                  `json:"unregistered"`
}

// RegisteredNodeCounts counts nodes that have registered with the API server.
type RegisteredNodeCounts struct {
	Total        int               `json:"total"`
	Ready        int               `json:"ready"`
	NotStarted   int               `json:"notStarted"`
	BeingDeleted int               `json:"beingDeleted"`
	Unready      UnreadyNodeCounts `json:"unready"`
}

// UnreadyNodeCounts splits unready nodes from those unready for lack of resources.
type UnreadyNodeCounts struct {
	Total           int `json:"total"`
	ResourceUnready int `json:"resourceUnready"`
}
