# AGENTS.md

Guidance for AI agents and new contributors working in this repository.

## What this is

A Prometheus exporter that publishes the Kubernetes `cluster-autoscaler-status` ConfigMap as
metrics. It exists for clusters whose cluster-autoscaler does not expose `/metrics` — managed
control planes such as GKE — where that ConfigMap is the only machine-readable view of autoscaler
state. Its reason for existing is `node_group_max_size` beside `node_group_target_size`: nothing
else lets you alert on "this node pool has reached its ceiling and cannot scale further".

Where the autoscaler does expose its own metrics, those are richer and this exporter is
unnecessary.

## Commands

```shell
make test      # lint + unit
make unit      # go test -v -cover ./...
make lint      # golangci-lint (version pinned in the Makefile)
make fmt       # golangci-lint fmt
make build     # go build ./...
make tools     # install the pinned golangci-lint and goreleaser
make snapshot  # goreleaser build --snapshot --clean
```

Requires Go 1.27+. Run the exporter without a cluster by pointing it at a captured document:

```shell
go run . --status.file=internal/collector/testdata/unhealthy-backoff.yaml
```

## Layout

| Path | Responsibility |
|---|---|
| `main.go` | Flags, source selection, registration, serving. Wiring only |
| `internal/status/` | The document model, `Parse`, and the known state-set values. No Kubernetes client, no Prometheus |
| `internal/source/` | The two adapters: ConfigMap (client-go) and file. Return raw bytes, never parsed structs |
| `internal/collector/` | The custom collector: descriptors, emit helpers, `Describe`/`Collect` |

The `Source` interface (`Fetch(ctx) ([]byte, error)`) is declared in `internal/collector` — the
consumer side — so `internal/source` imports nothing from it. Parsing lives in exactly one place
regardless of where the document came from.

## Settled design decisions

These were decided deliberately. Do not change them without a reason that addresses the rationale;
do flag any code that fails to live up to one.

- **Const metrics only.** Everything is built inside `Collect` with `MustNewConstMetric` and
  discarded. No package-level `Gauge`/`Counter` objects, no caching, no cross-scrape state, no
  mutexes, no informer. A scrape reads fresh data or reports failure.
- **`up` is the entire failure surface.** A fetch or parse failure emits
  `cluster_autoscaler_status_up 0` and nothing else. A field-level problem — an unreadable document
  timestamp — skips that one series and leaves `up` at 1. Never serve stale values.
- **`node_group` is the provider's identifier verbatim.** No URL parsing, no zone or pool
  extraction, no shortening. Encoding one cloud's naming scheme would make the exporter
  provider-specific; shortening belongs in Prometheus `metric_relabel_configs`, where it can change
  without a release.
- **Enum conditions are state sets**: `_state` suffix, `state` label, every known value emitted each
  scrape (`1` for the current one, `0` for the rest), and a value outside the known list gets its own
  series at `1` so a new upstream state is never swallowed.
- **Metric prefix `cluster_autoscaler_status_`**, deliberately distinct from a real
  cluster-autoscaler's own `cluster_autoscaler_` metrics so the two can never collide.
- **Free-form strings are never label values.** `message` and `backoffInfo.errorMessage` are
  unbounded; log them instead. Only add a label whose value set you can bound.
- **Decoding is lenient about unknown fields**, so an autoscaler upgrade that adds a field cannot
  break scraping. A decode error, or an empty `autoscalerStatus`, is a hard failure.
- **One `GET` per scrape.** No informer and no TTL cache: freshness is guaranteed and a failure is
  immediately visible as `up 0`.

## Conventions

- **Comments explain _why_, never _what_.** Default to no comment. Never narrate history ("used to",
  "previously"). A comment that paraphrases the code should be deleted.
- Conventional commit messages (`feat:`, `fix:`, `test:`, `docs:`, `chore:`, `refactor:`).
- Keep the linter lists in `.golangci.yaml` alphabetized. `promlinter` is enabled, and
  `TestCollectPassesPromlint` runs Prometheus's own linter over the real exposition output — a badly
  named metric fails the suite, not review.
- New metrics need a descriptor, an entry in `descriptors` (so `Describe` stays in sync), and a
  golden-file update.

## Tests

Standard library `testing`, with `testutil.CollectAndCompare` against golden `.prom` files. Write the
test before the implementation.

**A golden file must record what the code _should_ emit, not what it did.** Generate it from a
failing run, then check every line against the fixture before committing. The same applies to
fixtures: give fields distinct values, because a fixture where two fields share a value cannot tell
them apart — that is how a swapped label or a mis-sourced field survives a green suite. When a test
is meant to pin a behaviour, mutate the code and confirm the test actually fails.

The fixtures in `internal/status/testdata/` and `internal/collector/testdata/` are deliberately
duplicated copies; keep them byte-identical.

## Publishing rules

- **No real infrastructure identifiers, anywhere** — no real cloud project IDs, cluster names,
  instance-group names, zones, internal hostnames or IP addresses, in code, fixtures, docs or commit
  messages. Examples use `example-project`, `gke-demo-*`, `europe-west1-*`. Never commit a status
  ConfigMap captured from a live cluster; sanitize it first.
- **`docs/` and `.superpowers/` are gitignored** and hold local design notes. They must never be
  committed; CI fails the build if anything under either path is tracked, and `.dockerignore` keeps
  them out of the image build context. `README.md` is the only documentation this repository ships.
- Releases are tag-triggered (`v*`): goreleaser publishes binaries, buildx publishes a multi-arch
  image. The version string is stripped of its leading `v` so the binary and the image agree, and
  `latest` moves only for tags without a pre-release suffix.
