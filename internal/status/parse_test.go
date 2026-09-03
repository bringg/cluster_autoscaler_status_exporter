package status

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()

	raw, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	return raw
}

func TestParseHealthyDocument(t *testing.T) {
	s, err := Parse(readFixture(t, "gke-healthy.yaml"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if s.AutoscalerStatus != "Running" {
		t.Errorf("AutoscalerStatus = %q, want Running", s.AutoscalerStatus)
	}

	if got, want := len(s.NodeGroups), 2; got != want {
		t.Fatalf("len(NodeGroups) = %d, want %d", got, want)
	}

	wantCounts := NodeCounts{
		Registered: RegisteredNodeCounts{Total: 9, Ready: 9},
	}
	if diff := cmp.Diff(wantCounts, s.ClusterWide.Health.NodeCounts); diff != "" {
		t.Errorf("cluster-wide node counts mismatch (-want +got):\n%s", diff)
	}

	ng := s.NodeGroups[1]
	if ng.Health.MaxSize != 5 || ng.Health.MinSize != 2 || ng.Health.CloudProviderTarget != 2 {
		t.Errorf("node group sizes = min %d max %d target %d, want 2/5/2",
			ng.Health.MinSize, ng.Health.MaxSize, ng.Health.CloudProviderTarget)
	}

	wantProbe := time.Date(2026, 9, 2, 18, 33, 2, 0, time.UTC)
	if !ng.Health.LastProbeTime.Time.Equal(wantProbe) {
		t.Errorf("LastProbeTime = %v, want %v", ng.Health.LastProbeTime.Time, wantProbe)
	}
}

func TestParseUnhealthyDocument(t *testing.T) {
	s, err := Parse(readFixture(t, "unhealthy-backoff.yaml"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	wantClusterWideCounts := NodeCounts{
		Registered:       RegisteredNodeCounts{Total: 12, Ready: 7, NotStarted: 3, BeingDeleted: 1, Unready: UnreadyNodeCounts{Total: 4, ResourceUnready: 2}},
		LongUnregistered: 2,
		Unregistered:     5,
	}
	if diff := cmp.Diff(wantClusterWideCounts, s.ClusterWide.Health.NodeCounts); diff != "" {
		t.Errorf("cluster-wide node counts mismatch (-want +got):\n%s", diff)
	}

	if s.ClusterWide.Health.Status != "Unhealthy" {
		t.Errorf("cluster-wide Health.Status = %q, want Unhealthy", s.ClusterWide.Health.Status)
	}

	if s.ClusterWide.ScaleUp.Status != "InProgress" {
		t.Errorf("cluster-wide ScaleUp.Status = %q, want InProgress", s.ClusterWide.ScaleUp.Status)
	}

	if s.ClusterWide.ScaleDown.Status != "SomethingNew" {
		t.Errorf("cluster-wide ScaleDown.Status = %q, want SomethingNew", s.ClusterWide.ScaleDown.Status)
	}

	ng := s.NodeGroups[0]
	wantNodeGroupCounts := NodeCounts{
		Registered:       RegisteredNodeCounts{Total: 5, Ready: 3, NotStarted: 1, BeingDeleted: 1, Unready: UnreadyNodeCounts{Total: 1, ResourceUnready: 1}},
		LongUnregistered: 1,
		Unregistered:     2,
	}
	if diff := cmp.Diff(wantNodeGroupCounts, ng.Health.NodeCounts); diff != "" {
		t.Errorf("node group counts mismatch (-want +got):\n%s", diff)
	}

	if ng.Health.Status != "Unhealthy" {
		t.Errorf("node group Health.Status = %q, want Unhealthy", ng.Health.Status)
	}

	if ng.ScaleUp.Status != "Backoff" {
		t.Errorf("node group ScaleUp.Status = %q, want Backoff", ng.ScaleUp.Status)
	}

	if ng.ScaleDown.Status != "CandidatesPresent" {
		t.Errorf("node group ScaleDown.Status = %q, want CandidatesPresent", ng.ScaleDown.Status)
	}

	if ng.ScaleUp.BackoffInfo.ErrorCode != "QUOTA_EXCEEDED" {
		t.Errorf("ErrorCode = %q, want QUOTA_EXCEEDED", ng.ScaleUp.BackoffInfo.ErrorCode)
	}

	if ng.ScaleDown.Candidates != 3 {
		t.Errorf("ScaleDown.Candidates = %d, want 3", ng.ScaleDown.Candidates)
	}
}

func TestParseIgnoresUnknownFields(t *testing.T) {
	raw := []byte("autoscalerStatus: Running\nsomethingNew: 42\nclusterWide:\n  health:\n    status: Healthy\n")

	s, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if s.ClusterWide.Health.Status != "Healthy" {
		t.Errorf("Health.Status = %q, want Healthy", s.ClusterWide.Health.Status)
	}
}

func TestParseRejectsBadInput(t *testing.T) {
	tests := map[string][]byte{
		"malformed yaml": readFixture(t, "malformed.yaml"),
	}

	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(raw); err == nil {
				t.Fatal("Parse() error = nil, want an error")
			}
		})
	}
}

func TestParseEmptyStatusIsTyped(t *testing.T) {
	_, err := Parse([]byte("clusterWide: {}\n"))
	if !errors.Is(err, ErrEmptyStatus) {
		t.Fatalf("Parse() error = %v, want ErrEmptyStatus", err)
	}
}

func TestDocumentTime(t *testing.T) {
	s, err := Parse(readFixture(t, "gke-healthy.yaml"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	got, err := s.DocumentTime()
	if err != nil {
		t.Fatalf("DocumentTime() error = %v", err)
	}

	want := time.Date(2026, 9, 2, 18, 33, 2, 234927316, time.UTC)
	if !got.Equal(want) {
		t.Errorf("DocumentTime() = %v, want %v", got, want)
	}
}

func TestDocumentTimeMissing(t *testing.T) {
	s, err := Parse([]byte("autoscalerStatus: Running\n"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if _, err := s.DocumentTime(); err == nil {
		t.Fatal("DocumentTime() error = nil, want an error")
	}
}
