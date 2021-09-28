package kubernetes

import (
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// NewNamespace returns a new namespace/project
func NewNamespace(workshop *workshopv1.Workshop, scheme *runtime.Scheme, name string) *corev1.Namespace {

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	return namespace
}
