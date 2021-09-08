package certmanager

import (
	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// NewCustomResource create a Custom Resource
func NewCustomResource(workshop *workshopv1.Workshop, scheme *runtime.Scheme,
	name string, namespace string, labels map[string]string) *CertManager {

	cr := &CertManager{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: CertManagerSpec{},
	}

	// Set Workshop instance as the owner and controller
	err := ctrl.SetControllerReference(workshop, cr, scheme)
	if err != nil {
		log.Error(err, "Failed to set SetControllerReference")
	}
	return cr
}
