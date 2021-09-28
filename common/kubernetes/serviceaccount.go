package kubernetes

import (
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// NewServiceAccount creates a Service Account
func NewServiceAccount(workshop *workshopv1.Workshop, scheme *runtime.Scheme,
	name string, namespace string, labels map[string]string) *corev1.ServiceAccount {

	serviceaccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
	}

	// Set Workshop instance as the owner and controller
	/**
	Error: cross-namespace owner references are disallowed
	err := ctrl.SetControllerReference(workshop, serviceaccount, scheme)
	if err != nil {
		log.Error(err, " - Failed to set SetControllerReference for Service Account - %s", name)
	}
	*/

	return serviceaccount
}

// GetServiceAccount return Service Account
func GetServiceAccount(name string, namespace string, labels map[string]string) *corev1.ServiceAccount {

	serviceaccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
	}
	return serviceaccount
}
