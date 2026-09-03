package collector

import (
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bringg/cluster_autoscaler_status_exporter/internal/status"
)

const namespace = "cluster_autoscaler_status"

var (
	upDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "up"),
		"Whether the status document was fetched and parsed successfully.",
		nil, nil,
	)

	documentTimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "document_timestamp_seconds"),
		"Time the cluster-autoscaler wrote the status document, in seconds since the epoch.",
		nil, nil,
	)

	autoscalerStateDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "autoscaler_state"),
		"State of the cluster-autoscaler itself, 1 for the current state.",
		[]string{"state"}, nil,
	)

	healthStateDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "health_state"),
		"Cluster-wide health condition, 1 for the current state.",
		[]string{"state"}, nil,
	)

	scaleUpStateDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "scale_up_state"),
		"Cluster-wide scale-up condition, 1 for the current state.",
		[]string{"state"}, nil,
	)

	scaleDownStateDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "scale_down_state"),
		"Cluster-wide scale-down condition, 1 for the current state.",
		[]string{"state"}, nil,
	)

	scaleDownCandidatesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "scale_down_candidates"),
		"Number of nodes the cluster-autoscaler considers candidates for scale-down.",
		nil, nil,
	)

	nodesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "nodes"),
		"Nodes known to the cluster-autoscaler, by state.",
		[]string{"state"}, nil,
	)

	lastProbeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "last_probe_timestamp_seconds"),
		"Time the cluster-autoscaler last evaluated a cluster-wide condition, in seconds since the epoch.",
		[]string{"condition"}, nil,
	)

	lastTransitionDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "last_transition_timestamp_seconds"),
		"Time a cluster-wide condition last changed state, in seconds since the epoch.",
		[]string{"condition"}, nil,
	)

	nodeGroupMinSizeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node_group", "min_size"),
		"Smallest number of nodes the cluster-autoscaler may scale the node group to.",
		[]string{"node_group"}, nil,
	)

	nodeGroupMaxSizeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node_group", "max_size"),
		"Largest number of nodes the cluster-autoscaler may scale the node group to.",
		[]string{"node_group"}, nil,
	)

	nodeGroupTargetSizeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node_group", "target_size"),
		"Number of nodes the cloud provider is currently asked to run for the node group.",
		[]string{"node_group"}, nil,
	)

	nodeGroupNodesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node_group", "nodes"),
		"Nodes in the node group, by state.",
		[]string{"node_group", "state"}, nil,
	)

	nodeGroupHealthStateDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node_group", "health_state"),
		"Node group health condition, 1 for the current state.",
		[]string{"node_group", "state"}, nil,
	)

	nodeGroupScaleUpStateDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node_group", "scale_up_state"),
		"Node group scale-up condition, 1 for the current state.",
		[]string{"node_group", "state"}, nil,
	)

	nodeGroupScaleDownStateDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node_group", "scale_down_state"),
		"Node group scale-down condition, 1 for the current state.",
		[]string{"node_group", "state"}, nil,
	)

	nodeGroupScaleDownCandidatesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node_group", "scale_down_candidates"),
		"Nodes in the node group the cluster-autoscaler considers candidates for scale-down.",
		[]string{"node_group"}, nil,
	)

	nodeGroupScaleUpBackoffDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node_group", "scale_up_backoff"),
		"Present while the cluster-autoscaler is backing off from scaling the node group up, labelled with the cloud provider error code.",
		[]string{"node_group", "error_code"}, nil,
	)

	nodeGroupLastProbeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node_group", "last_probe_timestamp_seconds"),
		"Time the cluster-autoscaler last evaluated a node group condition, in seconds since the epoch.",
		[]string{"node_group", "condition"}, nil,
	)

	nodeGroupLastTransitionDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "node_group", "last_transition_timestamp_seconds"),
		"Time a node group condition last changed state, in seconds since the epoch.",
		[]string{"node_group", "condition"}, nil,
	)
)

// descriptors is every descriptor the collector can emit, so that Describe does
// not have to run a scrape to answer.
var descriptors = []*prometheus.Desc{
	upDesc,
	documentTimeDesc,
	autoscalerStateDesc,
	healthStateDesc,
	scaleUpStateDesc,
	scaleDownStateDesc,
	scaleDownCandidatesDesc,
	nodesDesc,
	lastProbeDesc,
	lastTransitionDesc,
	nodeGroupMinSizeDesc,
	nodeGroupMaxSizeDesc,
	nodeGroupTargetSizeDesc,
	nodeGroupNodesDesc,
	nodeGroupHealthStateDesc,
	nodeGroupScaleUpStateDesc,
	nodeGroupScaleDownStateDesc,
	nodeGroupScaleDownCandidatesDesc,
	nodeGroupScaleUpBackoffDesc,
	nodeGroupLastProbeDesc,
	nodeGroupLastTransitionDesc,
}

// stateSet emits every known state so a state that is no longer current reads
// as 0 rather than vanishing, and reports an unrecognised state on its own
// series so a new autoscaler state is never silently dropped.
func stateSet(ch chan<- prometheus.Metric, desc *prometheus.Desc, known []string, current string, labels ...string) {
	matched := false

	for _, state := range known {
		value := 0.0
		if state == current {
			value, matched = 1.0, true
		}

		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, appendLabel(labels, state)...)
	}

	if !matched && current != "" {
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, 1, appendLabel(labels, current)...)
	}
}

func nodeCountMetrics(ch chan<- prometheus.Metric, desc *prometheus.Desc, counts status.NodeCounts, labels ...string) {
	states := []struct {
		name  string
		value int
	}{
		{"registered", counts.Registered.Total},
		{"ready", counts.Registered.Ready},
		{"not_started", counts.Registered.NotStarted},
		{"being_deleted", counts.Registered.BeingDeleted},
		{"unready", counts.Registered.Unready.Total},
		{"unready_resource", counts.Registered.Unready.ResourceUnready},
		{"long_unregistered", counts.LongUnregistered},
		{"unregistered", counts.Unregistered},
	}

	for _, state := range states {
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(state.value), appendLabel(labels, state.name)...)
	}
}

// conditionTimeMetrics skips zero timestamps, which is how the autoscaler
// represents a condition it has not evaluated yet.
func conditionTimeMetrics(ch chan<- prometheus.Metric, probeDesc, transitionDesc *prometheus.Desc, condition string, probe, transition metav1.Time, labels ...string) {
	if !probe.IsZero() {
		ch <- prometheus.MustNewConstMetric(probeDesc, prometheus.GaugeValue, float64(probe.Unix()), appendLabel(labels, condition)...)
	}

	if !transition.IsZero() {
		ch <- prometheus.MustNewConstMetric(transitionDesc, prometheus.GaugeValue, float64(transition.Unix()), appendLabel(labels, condition)...)
	}
}

func appendLabel(labels []string, extra string) []string {
	values := make([]string, 0, len(labels)+1)
	values = append(values, labels...)

	return append(values, extra)
}
