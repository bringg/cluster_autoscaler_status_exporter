package source

import (
	"context"
	"strings"
	"testing"
)

func TestFileFetch(t *testing.T) {
	raw, err := NewFile("testdata/status.yaml").Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if !strings.Contains(string(raw), "autoscalerStatus: Running") {
		t.Errorf("Fetch() = %q, want it to contain the status document", raw)
	}
}

func TestFileFetchMissing(t *testing.T) {
	if _, err := NewFile("testdata/does-not-exist.yaml").Fetch(context.Background()); err == nil {
		t.Fatal("Fetch() error = nil, want an error")
	}
}
