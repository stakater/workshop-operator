package kubernetes

import (
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// NewConfigMap creates a ConfigMap
func NewConfigMap(workshop *workshopv1.Workshop, scheme *runtime.Scheme,
	name string, namespace string, labels map[string]string, data map[string]string) *corev1.ConfigMap {

	configmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Data: data,
	}
	return configmap
}
