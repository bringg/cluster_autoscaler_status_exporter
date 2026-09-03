package status

// The values the cluster-autoscaler writes for each condition, from its
// clusterstate/api package. A document may carry a value outside these lists
// after an autoscaler upgrade, which callers are expected to handle.
var (
	AutoscalerStates = []string{"Running", "Initializing"}
	HealthStates     = []string{"Healthy", "Unhealthy"}
	ScaleUpStates    = []string{"Needed", "NotNeeded", "InProgress", "NoActivity", "Backoff"}
	ScaleDownStates  = []string{"CandidatesPresent", "NoCandidates"}
)
