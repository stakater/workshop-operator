package user

import (
	userv1 "github.com/openshift/api/user/v1"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// NewUser create an user
func NewUser(workshop *workshopv1.Workshop, scheme *runtime.Scheme, username string, labels map[string]string) *userv1.User {

	user := &userv1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:   username,
			Labels: labels,
		},
		FullName: username,
	}
	return user
}

// NewUserRoleBinding create a RoleBinding for user
func NewUserRoleBinding(workshop *workshopv1.Workshop, scheme *runtime.Scheme, username string, namespace string,
	roleName string, roleKind string) *rbac.RoleBinding {

	roleBinding := &rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      username,
			Namespace: namespace,
		},
		Subjects: []rbac.Subject{
			{
				Kind:     "User",
				APIGroup: "rbac.authorization.k8s.io",
				Name:     username,
			},
		},
		RoleRef: rbac.RoleRef{
			Name: roleName,
			Kind: roleKind,
		},
	}
	return roleBinding
}

// NewHTPasswdSecret create a HTPasswd Secret
func NewHTPasswdSecret(workshop *workshopv1.Workshop, scheme *runtime.Scheme, name string, namespace string, htpasswds []byte) *corev1.Secret {

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: "Opaque",
		Data: map[string][]byte{
			"htpasswd": htpasswds,
		},
	}

	return secret
}

// NewIdentity create an identity for user
func NewIdentity(workshop *workshopv1.Workshop, scheme *runtime.Scheme, username string, identityName string, userFound *userv1.User) *userv1.Identity {

	identity := &userv1.Identity{
		ObjectMeta: metav1.ObjectMeta{
			Name: identityName + ":" + username,
		},
		ProviderName:     identityName,
		ProviderUserName: username,
		User: corev1.ObjectReference{
			Name: username,
			UID:  userFound.UID,
		},
	}
	return identity
}

// NewUserIdentityMapping create an user identity mapping for user
func NewUserIdentityMapping(workshop *workshopv1.Workshop, scheme *runtime.Scheme, userIdentityName, username string) *userv1.UserIdentityMapping {

	userIdentity := &userv1.UserIdentityMapping{
		ObjectMeta: metav1.ObjectMeta{
			Name: userIdentityName + ":" + username,
		},
		Identity: corev1.ObjectReference{
			Name: userIdentityName + ":" + username,
		},
		User: corev1.ObjectReference{
			Name: username,
		},
	}
	return userIdentity
}
