package kubernetes

import (
	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	admissionregistration "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
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

	// Set Workshop instance as the owner and controller
	err := ctrl.SetControllerReference(workshop, mwc, scheme)
	if err != nil {
		log.Error(err, "Failed to set SetControllerReference")
	}
	return mwc
}
