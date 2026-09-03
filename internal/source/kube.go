package source

import (
	"fmt"

	"github.com/prometheus/common/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// NewKubeClient builds a client from the in-cluster service account, falling
// back to the standard kubeconfig lookup so the exporter can be run locally.
func NewKubeClient(kubeconfigPath string) (kubernetes.Interface, error) {
	cfg, err := restConfig(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	cfg.UserAgent = version.PrometheusUserAgent()

	return kubernetes.NewForConfig(cfg)
}

// restConfig tries the pod's own service account before anything else, because
// clientcmd's deferred loader would otherwise let any kubeconfig that happens
// to exist in the image outrank the identity the exporter is deployed with.
func restConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath == "" {
		if cfg, err := rest.InClusterConfig(); err == nil {
			return cfg, nil
		}
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		loadingRules.ExplicitPath = kubeconfigPath
	}

	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("build kubernetes client config: %w", err)
	}

	return cfg, nil
}
