package kubernetes

import (
	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// NewClusterRole creates a ClusterRole
func NewClusterRole(workshop *workshopv1.Workshop, scheme *runtime.Scheme,
	name string, namespace string, labels map[string]string, rules []rbac.PolicyRule) *rbac.ClusterRole {

	clusterrole := &rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Rules: rules,
	}

	// Set Workshop instance as the owner and controller
	err := ctrl.SetControllerReference(workshop, clusterrole, scheme)
	if err != nil {
		log.Error(err, " - Failed to set SetControllerReference for ClusterRole.")
	}
	return clusterrole
}
