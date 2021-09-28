package kubernetes

import (
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewClusterRole returns a ClusterRole
func NewClusterRole(name string, namespace string, labels map[string]string, rules []rbac.PolicyRule) *rbac.ClusterRole {

	clusterrole := &rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Rules: rules,
	}
	return clusterrole
}