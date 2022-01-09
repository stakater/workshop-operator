package user

import (
	"fmt"
	userv1 "github.com/openshift/api/user/v1"
	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// NewUser creates a User
func NewUser(workshop *workshopv1.Workshop, scheme *runtime.Scheme, username string) *userv1.User {

	user := &userv1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: username,
		},
		FullName:   username,
		Identities: []string{"htpasswd:user1"},
	}
	return user
}

// NewRoleBindingUsers creates a Role Binding for Users
func NewRoleBindingUsers(workshop *workshopv1.Workshop, scheme *runtime.Scheme, username string, namespace string,
	roleName string, roleKind string) *rbac.RoleBinding {

	rolebinding := &rbac.RoleBinding{
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
	return rolebinding
}

// NewHTPasswdSecret create a HTPasswd Secret
func NewHTPasswdSecret(workshop *workshopv1.Workshop, scheme *runtime.Scheme, username string) *corev1.Secret {

	filedata, err := ioutil.ReadFile("hack/users.htpasswd")
	fmt.Println(string(filedata))
	if err != nil {
		log.Errorf(err.Error())
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "htpass-secret" + username,
			Namespace: "openshift-config",
		},
		Type: "Opaque",
		Data: map[string][]byte{
			"htpasswd": filedata,
		},
	}

	return secret
}

// NewIdentity creates a identity
func NewIdentity(workshop *workshopv1.Workshop, scheme *runtime.Scheme, username string, userFound *userv1.User) *userv1.Identity {

	identity := &userv1.Identity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "htpass-secret-" + ":" + username,
		},
		ProviderName:     "htpass-secret-",
		ProviderUserName: username,
		User: corev1.ObjectReference{
			Name: username,
			UID:  userFound.UID,
		},
	}
	return identity
}

// NewUserIdentity creates a useridentity
func NewUserIdentity(workshop *workshopv1.Workshop, scheme *runtime.Scheme, username string) *userv1.UserIdentityMapping {

	useridentity := &userv1.UserIdentityMapping{
		ObjectMeta: metav1.ObjectMeta{
			Name: "htpass-secret-" + ":" + username,
		},
		Identity: corev1.ObjectReference{
			Name: "htpass-secret-" + ":" + username,
		},
		User: corev1.ObjectReference{
			Name: username,
		},
	}
	return useridentity
}
