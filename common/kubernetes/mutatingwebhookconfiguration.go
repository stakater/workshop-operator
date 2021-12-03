package kubernetes

import (
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	admissionregistration "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// NewMutatingWebhookConfiguration creates a Mutating Webhook Configuration
func NewMutatingWebhookConfiguration(workshop *workshopv1.Workshop, scheme *runtime.Scheme,
	name string, labels map[string]string, webhooks []admissionregistration.MutatingWebhook) *admissionregistration.MutatingWebhookConfiguration {

	mwc := &admissionregistration.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Webhooks: webhooks,
	}
	return mwc
}
