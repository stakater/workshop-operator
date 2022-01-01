package user

import (
	"fmt"
	configv1 "github.com/openshift/api/config/v1"
	userv1 "github.com/openshift/api/user/v1"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	"io/ioutil"
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

// NewHTPasswd creates HTPasswd
func NewHTPasswd(workshop *workshopv1.Workshop, scheme *runtime.Scheme, username string) *configv1.OAuth {

	data, err := ioutil.ReadFile("hack/htpasswd")
	if err != nil {
		fmt.Println(err)
	}
	password := &configv1.OAuth{
		ObjectMeta: metav1.ObjectMeta{
			Name: "name",
		},
		Spec: configv1.OAuthSpec{
			IdentityProviders: []configv1.IdentityProvider{
				{
					Name:          "htpasswd" + username,
					MappingMethod: "claim",
					IdentityProviderConfig: configv1.IdentityProviderConfig{
						Type: "HTPasswd",
						HTPasswd: &configv1.HTPasswdIdentityProvider{
							FileData: configv1.SecretNameReference{
								Name: string(data),
							},
						},
					},
				},
			},
		},
	}
	return password
}
