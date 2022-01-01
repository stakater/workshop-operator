package user

import (
	bs64 "encoding/base64"
	configv1 "github.com/openshift/api/config/v1"
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
			"htpasswd": []byte(bs64.StdEncoding.EncodeToString(filedata)),
		},
	}

	return secret
}

// NewHTPasswd creates HTPasswd
func NewHTPasswd(workshop *workshopv1.Workshop, scheme *runtime.Scheme, username string) *configv1.OAuth {
	log.Infoln("NewHTPasswd")
	password := &configv1.OAuth{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
		Spec: configv1.OAuthSpec{
			IdentityProviders: []configv1.IdentityProvider{
				{
					Name:          "htpasswd",
					MappingMethod: "claim",
					IdentityProviderConfig: configv1.IdentityProviderConfig{
						Type: "HTPasswd",
						HTPasswd: &configv1.HTPasswdIdentityProvider{
							FileData: configv1.SecretNameReference{
								Name: "htpass-secret" + username,
							},
						},
					},
				},
			},
		},
	}
	return password
}
