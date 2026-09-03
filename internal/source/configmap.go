package source

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ConfigMap reads the status document from the ConfigMap the cluster-autoscaler
// maintains.
type ConfigMap struct {
	client    kubernetes.Interface
	namespace string
	name      string
	key       string
}

// NewConfigMap returns a source reading key from the named ConfigMap.
func NewConfigMap(client kubernetes.Interface, namespace, name, key string) *ConfigMap {
	return &ConfigMap{client: client, namespace: namespace, name: name, key: key}
}

// Fetch implements the collector's Source interface.
func (c *ConfigMap) Fetch(ctx context.Context) ([]byte, error) {
	cm, err := c.client.CoreV1().ConfigMaps(c.namespace).Get(ctx, c.name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get configmap %s/%s: %w", c.namespace, c.name, err)
	}

	raw, ok := cm.Data[c.key]
	if !ok {
		return nil, fmt.Errorf("configmap %s/%s has no %q key", c.namespace, c.name, c.key)
	}

	return []byte(raw), nil
}
