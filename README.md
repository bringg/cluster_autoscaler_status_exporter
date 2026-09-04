# Cluster Autoscaler Status Exporter

A Prometheus exporter for the Kubernetes `cluster-autoscaler-status` ConfigMap. The
cluster-autoscaler writes that ConfigMap on every loop with its own view of the world —
per-node-group bounds, health/scale-up/scale-down conditions, node counts, backoff state — and
this exporter turns it into Prometheus gauges.

**This exporter is for clusters whose cluster-autoscaler does not expose `/metrics`.** That's the
case on GKE and most other managed control planes, where the autoscaler runs inside the control
plane and nothing lets you scrape it directly. The ConfigMap is the only machine-readable signal
you get.

> If your cluster-autoscaler *does* expose `/metrics` (typical of a self-managed autoscaler running
> as a workload in your own cluster), scrape that instead and skip this exporter entirely. The
> autoscaler's native metrics are richer than anything recoverable from the status ConfigMap.
> Running both against the same autoscaler is redundant.

## Usage

```
$ go run . --help
usage: cluster_autoscaler_status_exporter [<flags>]


Flags:
  -h, --[no-]help                Show context-sensitive help (also try
                                 --help-long and --help-man).
      --configmap.namespace="kube-system"  
                                 Namespace of the cluster-autoscaler status
                                 ConfigMap.
      --configmap.name="cluster-autoscaler-status"  
                                 Name of the cluster-autoscaler status
                                 ConfigMap.
      --configmap.key="status"   Key inside the ConfigMap holding the status
                                 document.
      --status.file=STATUS.FILE  Read the status document from this file instead
                                 of the Kubernetes API.
      --kubernetes.kubeconfig=KUBERNETES.KUBECONFIG  
                                 Path to a kubeconfig file. Defaults to
                                 in-cluster credentials, then the standard
                                 kubeconfig lookup.
      --kubernetes.timeout=10s   Timeout for a single Kubernetes API request.
      --web.telemetry-path="/metrics"  
                                 Path under which to expose metrics.
      --[no-]web.systemd-socket  Use systemd socket activation listeners instead
                                 of port listeners (Linux only).
      --web.listen-address=:8080 ...  
                                 Addresses on which to expose metrics and web
                                 interface. Repeatable for multiple addresses.
                                 Examples: `:9100` or `[::1]:9100` for http,
                                 `vsock://:9100` for vsock
      --web.config.file=""       Path to configuration file that can
                                 enable TLS or authentication. See:
                                 https://github.com/prometheus/exporter-toolkit/blob/master/docs/web-configuration.md
      --log.level=info           Only log messages with the given severity or
                                 above. One of: [debug, info, warn, error]
      --log.format=logfmt        Output format of log messages. One of: [logfmt,
                                 json]
      --[no-]version             Show application version.
```

With no flags, the exporter reads in-cluster credentials (falling back to the standard kubeconfig
lookup), fetches the `status` key of the `cluster-autoscaler-status` ConfigMap in `kube-system`
on every scrape, and serves metrics on `:8080/metrics`.

## Metrics

Every metric is a gauge, rebuilt from scratch on each scrape — nothing is cached between scrapes,
so a failed fetch can never serve stale autoscaler state.

| Metric | Labels | Source field |
| --- | --- | --- |
| `cluster_autoscaler_status_up` | — | 1 if the status document was fetched and parsed successfully, 0 otherwise |
| `cluster_autoscaler_status_document_timestamp_seconds` | — | `time` |
| `cluster_autoscaler_status_autoscaler_state` | `state` | `autoscalerStatus` |
| `cluster_autoscaler_status_health_state` | `state` | `clusterWide.health.status` |
| `cluster_autoscaler_status_scale_up_state` | `state` | `clusterWide.scaleUp.status` |
| `cluster_autoscaler_status_scale_down_state` | `state` | `clusterWide.scaleDown.status` |
| `cluster_autoscaler_status_scale_down_candidates` | — | `clusterWide.scaleDown.candidates` |
| `cluster_autoscaler_status_nodes` | `state` | `clusterWide.health.nodeCounts` |
| `cluster_autoscaler_status_last_probe_timestamp_seconds` | `condition` | `clusterWide.*.lastProbeTime` |
| `cluster_autoscaler_status_last_transition_timestamp_seconds` | `condition` | `clusterWide.*.lastTransitionTime` |
| `cluster_autoscaler_status_node_group_min_size` | `node_group` | `nodeGroups[].health.minSize` |
| `cluster_autoscaler_status_node_group_max_size` | `node_group` | `nodeGroups[].health.maxSize` |
| `cluster_autoscaler_status_node_group_target_size` | `node_group` | `nodeGroups[].health.cloudProviderTarget` |
| `cluster_autoscaler_status_node_group_nodes` | `node_group`, `state` | `nodeGroups[].health.nodeCounts` |
| `cluster_autoscaler_status_node_group_health_state` | `node_group`, `state` | `nodeGroups[].health.status` |
| `cluster_autoscaler_status_node_group_scale_up_state` | `node_group`, `state` | `nodeGroups[].scaleUp.status` |
| `cluster_autoscaler_status_node_group_scale_down_state` | `node_group`, `state` | `nodeGroups[].scaleDown.status` |
| `cluster_autoscaler_status_node_group_scale_down_candidates` | `node_group` | `nodeGroups[].scaleDown.candidates` |
| `cluster_autoscaler_status_node_group_scale_up_backoff` | `node_group`, `error_code` | `nodeGroups[].scaleUp.backoffInfo.errorCode`, emitted only while backing off |
| `cluster_autoscaler_status_node_group_last_probe_timestamp_seconds` | `node_group`, `condition` | `nodeGroups[].*.lastProbeTime` |
| `cluster_autoscaler_status_node_group_last_transition_timestamp_seconds` | `node_group`, `condition` | `nodeGroups[].*.lastTransitionTime` |
| `cluster_autoscaler_status_exporter_build_info` | `version`, `revision`, `branch`, `goversion`, `goos`, `goarch`, `tags` | build metadata, always 1 |

Notes:

- `cluster_autoscaler_status_up` is the only metric emitted when a scrape fails — nothing else is
  published for that scrape, so an alert on `up == 0` is the way to detect a broken exporter or an
  unreadable ConfigMap.
- A `*_state`/`*_nodes` metric is emitted for **every** known value of its state set on every
  scrape, with `1` on the current value and `0` on the rest, so a state that stops being current
  reads as `0` rather than disappearing. If the autoscaler ever reports a value outside the known
  set (for example after an upstream upgrade), that value is emitted too, at `1` — it just isn't
  one of the rows below.
- `condition` is one of `health`, `scale_up`, `scale_down`. A probe/transition timestamp is omitted
  entirely if the autoscaler hasn't evaluated that condition yet (zero timestamp).
- `node_group` is the node group's name exactly as the cluster-autoscaler writes it — see
  [Shortening node group names](#shortening-node-group-names) below.

### Known state values

| Metric | `state` label values |
| --- | --- |
| `cluster_autoscaler_status_autoscaler_state` | `Running`, `Initializing` |
| `cluster_autoscaler_status_health_state`, `cluster_autoscaler_status_node_group_health_state` | `Healthy`, `Unhealthy` |
| `cluster_autoscaler_status_scale_up_state`, `cluster_autoscaler_status_node_group_scale_up_state` | `Needed`, `NotNeeded`, `InProgress`, `NoActivity`, `Backoff` |
| `cluster_autoscaler_status_scale_down_state`, `cluster_autoscaler_status_node_group_scale_down_state` | `CandidatesPresent`, `NoCandidates` |
| `cluster_autoscaler_status_nodes`, `cluster_autoscaler_status_node_group_nodes` | `registered`, `ready`, `not_started`, `being_deleted`, `unready`, `unready_resource`, `long_unregistered`, `unregistered` |

`node_group_scale_up_backoff`'s `error_code` label is not a fixed set — it's whatever error code
the cloud provider reported for the failed scale-up attempt.

## Example queries

```promql
# node groups sitting at their maximum size
cluster_autoscaler_status_node_group_target_size
  >= on(node_group) cluster_autoscaler_status_node_group_max_size

# autoscaler alive but not writing the status document
time() - cluster_autoscaler_status_document_timestamp_seconds > 300

# node groups backing off from scale-up
cluster_autoscaler_status_node_group_scale_up_backoff == 1
```

## Shortening node group names

The `node_group` label is the node group's name exactly as the cluster-autoscaler's cloud provider
implementation writes it, with no parsing or shortening applied. The exporter is provider-agnostic
by design — it has no notion of what a "cloud provider name" is supposed to look like, so it can't
safely truncate or reformat it.

In practice, on GCP that name is a full instance-group URL, for example:

```
https://www.googleapis.com/compute/v1/projects/example-project/zones/europe-west1-b/instanceGroups/gke-gke-demo-cluster-default-pool-1a2b3c4d-grp
```

That's unreadable in a dashboard or an alert. Shorten it in your scrape config with a
`metric_relabel_configs` rule instead of asking the exporter to guess at your provider's naming
scheme:

```yaml
metric_relabel_configs:
- source_labels: [__name__, node_group]
  regex: 'cluster_autoscaler_status_.*;.*/instanceGroups/(.+)'
  target_label: node_group
  replacement: '$1'
```

This rewrites `node_group` to just the trailing instance-group name (`gke-gke-demo-cluster-default-pool-1a2b3c4d-grp`
in the example above) on every metric this exporter produces, leaving other metrics untouched.

## Running in Kubernetes

Images are published to `ghcr.io/bringg/cluster_autoscaler_status_exporter`, tagged with each
release and `latest`, for `linux/amd64` and `linux/arm64`.

[`kubernetes.yaml`](kubernetes.yaml) deploys the exporter: a `ServiceAccount` in the `monitoring`
namespace, a `Role`/`RoleBinding` in `kube-system` that grants `get` on exactly one ConfigMap
(`cluster-autoscaler-status`, by resource name — nothing broader), and a single-replica
`Deployment` annotated for Prometheus scraping.

```shell
kubectl apply -f kubernetes.yaml
```

Adjust `--configmap.namespace`/`--configmap.name` (and the RBAC `Role`'s namespace/`resourceNames`
to match) if your cluster-autoscaler writes its status ConfigMap somewhere other than
`kube-system/cluster-autoscaler-status`.

## Development

```shell
make test    # lint + unit tests
make build   # compile
```

To try the exporter without cluster access, point it at a captured copy of the ConfigMap's
`status` document instead of the Kubernetes API:

```shell
kubectl get configmap cluster-autoscaler-status -n kube-system -o jsonpath='{.data.status}' > status.yaml
go run . --status.file=status.yaml
```
