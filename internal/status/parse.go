package status

import (
	"errors"
	"fmt"
	"time"

	"sigs.k8s.io/yaml"
)

// This matches upstream's ConfigMapLastUpdateFormat (clusterstate/utils/status.go),
// not time.Time.String() — re-check against that constant on an autoscaler upgrade.
const documentTimeLayout = "2006-01-02 15:04:05.999999999 -0700 MST"

// ErrEmptyStatus reports a document that decoded cleanly but carries no
// autoscaler status, which is how a wrong or truncated ConfigMap key shows up.
var ErrEmptyStatus = errors.New("status document has no autoscalerStatus")

// Parse decodes a status document, ignoring fields it does not model so that a
// cluster-autoscaler upgrade cannot break scraping.
func Parse(raw []byte) (*Status, error) {
	var s Status

	if err := yaml.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("decode status document: %w", err)
	}

	if s.AutoscalerStatus == "" {
		return nil, ErrEmptyStatus
	}

	return &s, nil
}

// DocumentTime is when the autoscaler wrote the document.
func (s *Status) DocumentTime() (time.Time, error) {
	return time.Parse(documentTimeLayout, s.Time)
}
