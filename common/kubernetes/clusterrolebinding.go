package kubernetes

import (
	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

// NewClusterRoleBindingSA creates a ClusterRoleBinding for Service Account
func NewClusterRoleBindingSA(workshop *workshopv1.Workshop, scheme *runtime.Scheme,
	name string, namespace string, labels map[string]string, serviceAccountName string, roleName string, roleKind string) *rbac.ClusterRoleBinding {

	clusterrolebinding := &rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Subjects: []rbac.Subject{
			{
				Kind:      rbac.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: namespace,
			},
		},
		RoleRef: rbac.RoleRef{
			Name:     roleName,
			Kind:     roleKind,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	// Set Workshop instance as the owner and controller
	err := ctrl.SetControllerReference(workshop, clusterrolebinding, scheme)
	if err != nil {
		log.Error(err, " - Failed to set SetControllerReference for ClusterRoleBinding for Service Account - %s", namespace)
	}
	return clusterrolebinding
}

// GetClusterRoleBindingSA return a ClusterRoleBinding for Service Account
func GetClusterRoleBindingSA(name string, namespace string, labels map[string]string, serviceAccountName string, roleName string, roleKind string) *rbac.ClusterRoleBinding {

	clusterrolebinding := &rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Subjects: []rbac.Subject{
			{
				Kind:      rbac.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: namespace,
			},
		},
		RoleRef: rbac.RoleRef{
			Name:     roleName,
			Kind:     roleKind,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	return clusterrolebinding
}

// NewClusterRoleBinding creates a ClusterRoleBinding for Users
func NewClusterRoleBinding(workshop *workshopv1.Workshop, scheme *runtime.Scheme,
	name string, namespace string, labels map[string]string, username string, roleName string, roleKind string) *rbac.ClusterRoleBinding {

	clusterrolebinding := &rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Subjects: []rbac.Subject{
			{
				Kind: rbac.UserKind,
				Name: username,
			},
		},
		RoleRef: rbac.RoleRef{
			Name:     roleName,
			Kind:     roleKind,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	// Set Workshop instance as the owner and controller
	/**
	Error: Cross-namespace owner references are disallowed
	err := ctrl.SetControllerReference(workshop, clusterrolebinding, scheme)

	err := ctrl.SetControllerReference(workshop, clusterrolebinding, scheme)
	if err != nil {
		log.Error(err, " - Failed to set SetControllerReference for ClusterRoleBinding for Users - %s", name)
	}
	**/
	return clusterrolebinding
}
