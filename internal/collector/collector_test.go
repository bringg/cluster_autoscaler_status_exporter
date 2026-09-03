package collector

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/promslog"
)

type stubSource struct {
	raw []byte
	err error
}

func (s stubSource) Fetch(context.Context) ([]byte, error) {
	return s.raw, s.err
}

func fixtureSource(t *testing.T, name string) stubSource {
	t.Helper()

	raw, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	return stubSource{raw: raw}
}

func newTestCollector(src Source) *Collector {
	return New(src, time.Second, promslog.NewNopLogger())
}

func TestCollectUpAndDocumentTime(t *testing.T) {
	c := newTestCollector(fixtureSource(t, "minimal.yaml"))

	expected := `
# HELP cluster_autoscaler_status_document_timestamp_seconds Time the cluster-autoscaler wrote the status document, in seconds since the epoch.
# TYPE cluster_autoscaler_status_document_timestamp_seconds gauge
cluster_autoscaler_status_document_timestamp_seconds 1.788373982e+09
# HELP cluster_autoscaler_status_up Whether the status document was fetched and parsed successfully.
# TYPE cluster_autoscaler_status_up gauge
cluster_autoscaler_status_up 1
`

	if err := testutil.CollectAndCompare(c, strings.NewReader(expected),
		"cluster_autoscaler_status_up",
		"cluster_autoscaler_status_document_timestamp_seconds",
	); err != nil {
		t.Fatal(err)
	}
}

func TestCollectSkipsZeroTimestamps(t *testing.T) {
	c := newTestCollector(fixtureSource(t, "minimal.yaml"))

	if got := testutil.CollectAndCount(c, "cluster_autoscaler_status_last_probe_timestamp_seconds"); got != 0 {
		t.Errorf("last_probe_timestamp_seconds series count = %d, want 0", got)
	}

	if got := testutil.CollectAndCount(c, "cluster_autoscaler_status_last_transition_timestamp_seconds"); got != 0 {
		t.Errorf("last_transition_timestamp_seconds series count = %d, want 0", got)
	}
}

func TestCollectFetchFailure(t *testing.T) {
	c := newTestCollector(stubSource{err: errors.New("api server unreachable")})

	expected := `
# HELP cluster_autoscaler_status_up Whether the status document was fetched and parsed successfully.
# TYPE cluster_autoscaler_status_up gauge
cluster_autoscaler_status_up 0
`

	if err := testutil.CollectAndCompare(c, strings.NewReader(expected)); err != nil {
		t.Fatal(err)
	}
}

func TestCollectParseFailure(t *testing.T) {
	c := newTestCollector(stubSource{raw: []byte("not: [valid")})

	expected := `
# HELP cluster_autoscaler_status_up Whether the status document was fetched and parsed successfully.
# TYPE cluster_autoscaler_status_up gauge
cluster_autoscaler_status_up 0
`

	if err := testutil.CollectAndCompare(c, strings.NewReader(expected)); err != nil {
		t.Fatal(err)
	}
}

func TestCollectBadDocumentTimeKeepsUp(t *testing.T) {
	c := newTestCollector(fixtureSource(t, "bad-timestamp.yaml"))

	// A field the exporter cannot read is not a failed scrape: the series is
	// skipped, up stays 1.
	expected := `
# HELP cluster_autoscaler_status_up Whether the status document was fetched and parsed successfully.
# TYPE cluster_autoscaler_status_up gauge
cluster_autoscaler_status_up 1
`

	if err := testutil.CollectAndCompare(c, strings.NewReader(expected),
		"cluster_autoscaler_status_up",
		"cluster_autoscaler_status_document_timestamp_seconds",
	); err != nil {
		t.Fatal(err)
	}
}

func TestCollectClusterWide(t *testing.T) {
	c := newTestCollector(fixtureSource(t, "unhealthy-backoff.yaml"))

	golden, err := os.Open("testdata/cluster-wide.golden.prom")
	if err != nil {
		t.Fatalf("open golden: %v", err)
	}
	defer golden.Close()

	if err := testutil.CollectAndCompare(c, golden,
		"cluster_autoscaler_status_autoscaler_state",
		"cluster_autoscaler_status_health_state",
		"cluster_autoscaler_status_scale_up_state",
		"cluster_autoscaler_status_scale_down_state",
		"cluster_autoscaler_status_scale_down_candidates",
		"cluster_autoscaler_status_nodes",
		"cluster_autoscaler_status_last_probe_timestamp_seconds",
		"cluster_autoscaler_status_last_transition_timestamp_seconds",
	); err != nil {
		t.Fatal(err)
	}
}

func TestCollectUnknownStateIsReported(t *testing.T) {
	c := newTestCollector(fixtureSource(t, "unhealthy-backoff.yaml"))

	// The fixture carries a scale-down status the autoscaler does not document,
	// which must still reach Prometheus rather than being dropped.
	expected := `
# HELP cluster_autoscaler_status_scale_down_state Cluster-wide scale-down condition, 1 for the current state.
# TYPE cluster_autoscaler_status_scale_down_state gauge
cluster_autoscaler_status_scale_down_state{state="CandidatesPresent"} 0
cluster_autoscaler_status_scale_down_state{state="NoCandidates"} 0
cluster_autoscaler_status_scale_down_state{state="SomethingNew"} 1
`

	if err := testutil.CollectAndCompare(c, strings.NewReader(expected),
		"cluster_autoscaler_status_scale_down_state",
	); err != nil {
		t.Fatal(err)
	}
}

func TestCollectNodeGroups(t *testing.T) {
	c := newTestCollector(fixtureSource(t, "unhealthy-backoff.yaml"))

	golden, err := os.Open("testdata/node-groups.golden.prom")
	if err != nil {
		t.Fatalf("open golden: %v", err)
	}
	defer golden.Close()

	if err := testutil.CollectAndCompare(c, golden,
		"cluster_autoscaler_status_node_group_min_size",
		"cluster_autoscaler_status_node_group_max_size",
		"cluster_autoscaler_status_node_group_target_size",
		"cluster_autoscaler_status_node_group_nodes",
		"cluster_autoscaler_status_node_group_health_state",
		"cluster_autoscaler_status_node_group_scale_up_state",
		"cluster_autoscaler_status_node_group_scale_down_state",
		"cluster_autoscaler_status_node_group_scale_down_candidates",
		"cluster_autoscaler_status_node_group_scale_up_backoff",
		"cluster_autoscaler_status_node_group_last_probe_timestamp_seconds",
		"cluster_autoscaler_status_node_group_last_transition_timestamp_seconds",
	); err != nil {
		t.Fatal(err)
	}
}

func TestCollectNodeGroupNameIsVerbatim(t *testing.T) {
	c := newTestCollector(fixtureSource(t, "gke-healthy.yaml"))

	expected := `
# HELP cluster_autoscaler_status_node_group_max_size Largest number of nodes the cluster-autoscaler may scale the node group to.
# TYPE cluster_autoscaler_status_node_group_max_size gauge
cluster_autoscaler_status_node_group_max_size{node_group="https://www.googleapis.com/compute/v1/projects/example-project/zones/europe-west1-b/instanceGroups/gke-demo-default-pool-9f1c2d3e-grp"} 4
cluster_autoscaler_status_node_group_max_size{node_group="https://www.googleapis.com/compute/v1/projects/example-project/zones/europe-west1-a/instanceGroups/gke-demo-monitoring-7a4b5c6d-grp"} 5
`

	if err := testutil.CollectAndCompare(c, strings.NewReader(expected),
		"cluster_autoscaler_status_node_group_max_size",
	); err != nil {
		t.Fatal(err)
	}
}

func TestCollectNoBackoffSeriesWhenNotBackingOff(t *testing.T) {
	c := newTestCollector(fixtureSource(t, "gke-healthy.yaml"))

	if got := testutil.CollectAndCount(c, "cluster_autoscaler_status_node_group_scale_up_backoff"); got != 0 {
		t.Errorf("backoff series count = %d, want 0", got)
	}
}

func TestCollectNodeGroupSizes(t *testing.T) {
	c := newTestCollector(fixtureSource(t, "gke-healthy.yaml"))

	expected := `
# HELP cluster_autoscaler_status_node_group_max_size Largest number of nodes the cluster-autoscaler may scale the node group to.
# TYPE cluster_autoscaler_status_node_group_max_size gauge
cluster_autoscaler_status_node_group_max_size{node_group="https://www.googleapis.com/compute/v1/projects/example-project/zones/europe-west1-a/instanceGroups/gke-demo-monitoring-7a4b5c6d-grp"} 5
cluster_autoscaler_status_node_group_max_size{node_group="https://www.googleapis.com/compute/v1/projects/example-project/zones/europe-west1-b/instanceGroups/gke-demo-default-pool-9f1c2d3e-grp"} 4
# HELP cluster_autoscaler_status_node_group_min_size Smallest number of nodes the cluster-autoscaler may scale the node group to.
# TYPE cluster_autoscaler_status_node_group_min_size gauge
cluster_autoscaler_status_node_group_min_size{node_group="https://www.googleapis.com/compute/v1/projects/example-project/zones/europe-west1-a/instanceGroups/gke-demo-monitoring-7a4b5c6d-grp"} 2
cluster_autoscaler_status_node_group_min_size{node_group="https://www.googleapis.com/compute/v1/projects/example-project/zones/europe-west1-b/instanceGroups/gke-demo-default-pool-9f1c2d3e-grp"} 1
# HELP cluster_autoscaler_status_node_group_target_size Number of nodes the cloud provider is currently asked to run for the node group.
# TYPE cluster_autoscaler_status_node_group_target_size gauge
cluster_autoscaler_status_node_group_target_size{node_group="https://www.googleapis.com/compute/v1/projects/example-project/zones/europe-west1-a/instanceGroups/gke-demo-monitoring-7a4b5c6d-grp"} 2
cluster_autoscaler_status_node_group_target_size{node_group="https://www.googleapis.com/compute/v1/projects/example-project/zones/europe-west1-b/instanceGroups/gke-demo-default-pool-9f1c2d3e-grp"} 1
`

	if err := testutil.CollectAndCompare(c, strings.NewReader(expected),
		"cluster_autoscaler_status_node_group_min_size",
		"cluster_autoscaler_status_node_group_max_size",
		"cluster_autoscaler_status_node_group_target_size",
	); err != nil {
		t.Fatal(err)
	}
}

func TestCollectSkipsDuplicateAndEmptyNodeGroupNames(t *testing.T) {
	c := newTestCollector(fixtureSource(t, "duplicate-node-groups.yaml"))

	expected := `
# HELP cluster_autoscaler_status_node_group_max_size Largest number of nodes the cluster-autoscaler may scale the node group to.
# TYPE cluster_autoscaler_status_node_group_max_size gauge
cluster_autoscaler_status_node_group_max_size{node_group="dup"} 3
# HELP cluster_autoscaler_status_up Whether the status document was fetched and parsed successfully.
# TYPE cluster_autoscaler_status_up gauge
cluster_autoscaler_status_up 1
`

	if err := testutil.CollectAndCompare(c, strings.NewReader(expected),
		"cluster_autoscaler_status_up",
		"cluster_autoscaler_status_node_group_max_size",
	); err != nil {
		t.Fatal(err)
	}
}

func TestCollectPassesPromlint(t *testing.T) {
	c := newTestCollector(fixtureSource(t, "unhealthy-backoff.yaml"))

	problems, err := testutil.CollectAndLint(c)
	if err != nil {
		t.Fatalf("CollectAndLint() error = %v", err)
	}

	for _, problem := range problems {
		t.Errorf("%s: %s", problem.Metric, problem.Text)
	}
}
