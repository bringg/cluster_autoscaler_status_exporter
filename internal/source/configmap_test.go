package source

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestConfigMapFetch(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-autoscaler-status", Namespace: "kube-system"},
		Data:       map[string]string{"status": "autoscalerStatus: Running\n"},
	})

	raw, err := NewConfigMap(client, "kube-system", "cluster-autoscaler-status", "status").Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if got, want := string(raw), "autoscalerStatus: Running\n"; got != want {
		t.Errorf("Fetch() = %q, want %q", got, want)
	}
}

func TestConfigMapFetchMissingConfigMap(t *testing.T) {
	client := fake.NewSimpleClientset()

	if _, err := NewConfigMap(client, "kube-system", "cluster-autoscaler-status", "status").Fetch(context.Background()); err == nil {
		t.Fatal("Fetch() error = nil, want an error")
	}
}

func TestConfigMapFetchMissingKey(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-autoscaler-status", Namespace: "kube-system"},
		Data:       map[string]string{"other": "..."},
	})

	_, err := NewConfigMap(client, "kube-system", "cluster-autoscaler-status", "status").Fetch(context.Background())
	if err == nil {
		t.Fatal("Fetch() error = nil, want an error")
	}
}
