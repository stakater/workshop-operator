package gitea

import (
	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// NewCustomResource create a new  CustomResource
func NewCustomResource(workshop *workshopv1.Workshop, scheme *runtime.Scheme,
	name string, namespace string, labels map[string]string) *Gitea {
	cr := &Gitea{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: GiteaSpec{
			GiteaVolumeSize:      "4Gi",
			GiteaSsl:             true,
			PostgresqlVolumeSize: "4Gi",
		},
	}

	// Set Workshop instance as the owner and controller
	err := ctrl.SetControllerReference(workshop, cr, scheme)
	if err != nil {
		log.Errorf("Failed to set SetControllerReference for %s with %s", "gitea CR", err)
	}
	return cr
}

// GetCustomResource return  CustomResource
func GetCustomResource(name string, namespace string, labels map[string]string) *Gitea {
	cr := &Gitea{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: GiteaSpec{
			GiteaVolumeSize:      "4Gi",
			GiteaSsl:             true,
			PostgresqlVolumeSize: "4Gi",
		},
	}
	return cr
}
