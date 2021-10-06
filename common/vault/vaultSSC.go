package vault

import (
	securityv1 "github.com/openshift/api/security/v1"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// NewVaultSSC return a VaultSSC
func NewVaultSSC(workshop *workshopv1.Workshop, scheme *runtime.Scheme,
	name string) *securityv1.SecurityContextConstraints {

	vaultSSC := &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		AllowPrivilegedContainer: true,
	}
	return vaultSSC
}

/**

oc describe scc privileged
Name:						privileged
Priority:					<none>
Access:
  Users:					system:admin,system:serviceaccount:openshift-infra:build-controller,system:serviceaccount:vault:vault
  Groups:					system:cluster-admins,system:nodes,system:masters

oc describe scc privileged
Name:						privileged
Priority:					<none>
Access:
  Users:					system:admin,system:serviceaccount:openshift-infra:build-controller
  Groups:					system:cluster-admins,system:nodes,system:masters


*/
