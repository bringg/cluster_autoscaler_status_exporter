package collector

import (
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"testing"
)

const dashboardPath = "../../grafana/dashboard.json"

var (
	fqNamePattern     = regexp.MustCompile(`fqName: "([^"]+)"`)
	metricNamePattern = regexp.MustCompile(namespace + `[a-z0-9_]*`)
)

// TestDashboardReferencesEmittedMetrics ties the shipped dashboard to the
// descriptors, because a dashboard names its metrics in strings that no
// compiler or linter will follow through a rename.
func TestDashboardReferencesEmittedMetrics(t *testing.T) {
	raw, err := os.ReadFile(dashboardPath)
	if err != nil {
		t.Fatalf("read dashboard: %v", err)
	}

	var document any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("parse dashboard: %v", err)
	}

	emitted := make(map[string]bool, len(descriptors))

	for _, desc := range descriptors {
		match := fqNamePattern.FindStringSubmatch(desc.String())
		if match == nil {
			t.Fatalf("no fqName in descriptor %s", desc)
		}

		emitted[match[1]] = true
	}

	referenced := make(map[string]bool)

	for _, query := range collectQueries(document) {
		for _, name := range metricNamePattern.FindAllString(query, -1) {
			referenced[name] = true
		}
	}

	if len(referenced) == 0 {
		t.Fatal("no exporter metrics found in the dashboard queries; the extraction is broken, not the dashboard")
	}

	for _, name := range sortedKeys(referenced) {
		if !emitted[name] {
			t.Errorf("dashboard queries %s, which the collector does not emit", name)
		}
	}
}

// collectQueries walks the document rather than modelling it, so that a panel
// or variable layout Grafana writes differently still has its queries checked.
func collectQueries(node any) []string {
	switch value := node.(type) {
	case map[string]any:
		var queries []string

		for key, child := range value {
			if text, ok := child.(string); ok && (key == "expr" || key == "query" || key == "definition") {
				queries = append(queries, text)

				continue
			}

			queries = append(queries, collectQueries(child)...)
		}

		return queries
	case []any:
		var queries []string

		for _, child := range value {
			queries = append(queries, collectQueries(child)...)
		}

		return queries
	default:
		return nil
	}
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}
