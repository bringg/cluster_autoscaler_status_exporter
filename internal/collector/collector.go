// Package collector turns a cluster-autoscaler status document into Prometheus
// metrics. Every metric is built during Collect and nothing is kept between
// scrapes, so a failed scrape can never serve stale autoscaler state.
package collector

import (
	"context"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bringg/cluster_autoscaler_status_exporter/internal/status"
)

// Source fetches the raw status document.
type Source interface {
	Fetch(ctx context.Context) ([]byte, error)
}

// Collector implements prometheus.Collector over a Source.
type Collector struct {
	source  Source
	timeout time.Duration
	logger  *slog.Logger
}

// New returns a collector reading from src, giving each fetch at most timeout.
func New(src Source, timeout time.Duration, logger *slog.Logger) *Collector {
	return &Collector{source: src, timeout: timeout, logger: logger}
}

// Describe implements prometheus.Collector.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range descriptors {
		ch <- desc
	}
}

// Collect implements prometheus.Collector.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	s, err := c.status(ctx)
	if err != nil {
		c.logger.Error("Scrape failed", "err", err)
		ch <- prometheus.MustNewConstMetric(upDesc, prometheus.GaugeValue, 0)

		return
	}

	ch <- prometheus.MustNewConstMetric(upDesc, prometheus.GaugeValue, 1)

	if documentTime, err := s.DocumentTime(); err != nil {
		c.logger.Warn("Unparseable document timestamp", "time", s.Time, "err", err)
	} else {
		ch <- prometheus.MustNewConstMetric(documentTimeDesc, prometheus.GaugeValue, float64(documentTime.Unix()))
	}

	c.collectClusterWide(ch, s)
	c.collectNodeGroups(ch, s)
}

func (c *Collector) collectClusterWide(ch chan<- prometheus.Metric, s *status.Status) {
	stateSet(ch, autoscalerStateDesc, status.AutoscalerStates, s.AutoscalerStatus)
	stateSet(ch, healthStateDesc, status.HealthStates, s.ClusterWide.Health.Status)
	stateSet(ch, scaleUpStateDesc, status.ScaleUpStates, s.ClusterWide.ScaleUp.Status)
	stateSet(ch, scaleDownStateDesc, status.ScaleDownStates, s.ClusterWide.ScaleDown.Status)

	ch <- prometheus.MustNewConstMetric(scaleDownCandidatesDesc, prometheus.GaugeValue, float64(s.ClusterWide.ScaleDown.Candidates))

	nodeCountMetrics(ch, nodesDesc, s.ClusterWide.Health.NodeCounts)

	conditionTimeMetrics(ch, lastProbeDesc, lastTransitionDesc, "health",
		s.ClusterWide.Health.LastProbeTime, s.ClusterWide.Health.LastTransitionTime)
	conditionTimeMetrics(ch, lastProbeDesc, lastTransitionDesc, "scale_up",
		s.ClusterWide.ScaleUp.LastProbeTime, s.ClusterWide.ScaleUp.LastTransitionTime)
	conditionTimeMetrics(ch, lastProbeDesc, lastTransitionDesc, "scale_down",
		s.ClusterWide.ScaleDown.LastProbeTime, s.ClusterWide.ScaleDown.LastTransitionTime)
}

func (c *Collector) collectNodeGroups(ch chan<- prometheus.Metric, s *status.Status) {
	seen := make(map[string]struct{}, len(s.NodeGroups))

	for _, ng := range s.NodeGroups {
		if ng.Name == "" {
			c.logger.Warn("Skipping node group with no name")

			continue
		}

		if _, ok := seen[ng.Name]; ok {
			c.logger.Warn("Skipping duplicate node group", "node_group", ng.Name)

			continue
		}

		seen[ng.Name] = struct{}{}

		ch <- prometheus.MustNewConstMetric(nodeGroupMinSizeDesc, prometheus.GaugeValue, float64(ng.Health.MinSize), ng.Name)
		ch <- prometheus.MustNewConstMetric(nodeGroupMaxSizeDesc, prometheus.GaugeValue, float64(ng.Health.MaxSize), ng.Name)
		ch <- prometheus.MustNewConstMetric(nodeGroupTargetSizeDesc, prometheus.GaugeValue, float64(ng.Health.CloudProviderTarget), ng.Name)
		ch <- prometheus.MustNewConstMetric(nodeGroupScaleDownCandidatesDesc, prometheus.GaugeValue, float64(ng.ScaleDown.Candidates), ng.Name)

		nodeCountMetrics(ch, nodeGroupNodesDesc, ng.Health.NodeCounts, ng.Name)

		stateSet(ch, nodeGroupHealthStateDesc, status.HealthStates, ng.Health.Status, ng.Name)
		stateSet(ch, nodeGroupScaleUpStateDesc, status.ScaleUpStates, ng.ScaleUp.Status, ng.Name)
		stateSet(ch, nodeGroupScaleDownStateDesc, status.ScaleDownStates, ng.ScaleDown.Status, ng.Name)

		if ng.ScaleUp.BackoffInfo.ErrorCode != "" {
			ch <- prometheus.MustNewConstMetric(nodeGroupScaleUpBackoffDesc, prometheus.GaugeValue, 1, ng.Name, ng.ScaleUp.BackoffInfo.ErrorCode)
			c.logger.Warn("Node group backing off scale-up",
				"node_group", ng.Name, "error_code", ng.ScaleUp.BackoffInfo.ErrorCode, "error_message", ng.ScaleUp.BackoffInfo.ErrorMessage)
		}

		conditionTimeMetrics(ch, nodeGroupLastProbeDesc, nodeGroupLastTransitionDesc, "health",
			ng.Health.LastProbeTime, ng.Health.LastTransitionTime, ng.Name)
		conditionTimeMetrics(ch, nodeGroupLastProbeDesc, nodeGroupLastTransitionDesc, "scale_up",
			ng.ScaleUp.LastProbeTime, ng.ScaleUp.LastTransitionTime, ng.Name)
		conditionTimeMetrics(ch, nodeGroupLastProbeDesc, nodeGroupLastTransitionDesc, "scale_down",
			ng.ScaleDown.LastProbeTime, ng.ScaleDown.LastTransitionTime, ng.Name)
	}
}

func (c *Collector) status(ctx context.Context) (*status.Status, error) {
	raw, err := c.source.Fetch(ctx)
	if err != nil {
		return nil, err
	}

	s, err := status.Parse(raw)
	if err != nil {
		c.logger.Debug("Unparseable status document", "document", string(raw))

		return nil, err
	}

	return s, nil
}
