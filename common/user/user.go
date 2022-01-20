package user

import (
	"bytes"
	"fmt"
	userv1 "github.com/openshift/api/user/v1"
	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"os/exec"
)

// NewUser creates a User
func NewUser(workshop *workshopv1.Workshop, scheme *runtime.Scheme, username string) *userv1.User {

	user := &userv1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: username,
		},
		FullName: username,
	}
	return user
}

// NewRoleBindingUsers creates a Role Binding for User
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
func NewHTPasswdSecret(workshop *workshopv1.Workshop, scheme *runtime.Scheme, htpasswd []byte) *corev1.Secret {

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "htpass-workshop-users",
			Namespace: "openshift-config",
		},
		Type: "Opaque",
		Data: map[string][]byte{
			"htpasswd": htpasswd,
		},
	}

	return secret
}

// NewIdentity creates a identity
func NewIdentity(workshop *workshopv1.Workshop, scheme *runtime.Scheme, username string, userFound *userv1.User) *userv1.Identity {

	identity := &userv1.Identity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "htpass-workshop-users" + ":" + username,
		},
		ProviderName:     "htpass-workshop-users",
		ProviderUserName: username,
		User: corev1.ObjectReference{
			Name: username,
			UID:  userFound.UID,
		},
	}
	return identity
}

// NewUserIdentity creates a user identity mapping
func NewUserIdentityMapping(workshop *workshopv1.Workshop, scheme *runtime.Scheme, username string) *userv1.UserIdentityMapping {

	useridentity := &userv1.UserIdentityMapping{
		ObjectMeta: metav1.ObjectMeta{
			Name: "htpass-workshop-users" + ":" + username,
		},
		Identity: corev1.ObjectReference{
			Name: "htpass-workshop-users" + ":" + username,
		},
		User: corev1.ObjectReference{
			Name: username,
		},
	}
	return useridentity
}

func GeneratePasswd(workshop *workshopv1.Workshop, username string) []byte {

	password := workshop.Spec.UserDetails.DefaultPassword
	shellScript, err := ioutil.ReadFile("/tmp/scripts/generate_htpasswd.sh")
	if err != nil {
		log.Errorf(err.Error())
	}

	shellScript = bytes.Replace(shellScript, []byte("username"), []byte(username), -1)
	shellScript = bytes.Replace(shellScript, []byte("password"), []byte(password), -1)
	if err = ioutil.WriteFile("/tmp/scripts/generate_htpasswd.sh", shellScript, 0644); err != nil {
		log.Fatal(err)
	}
	_, err = exec.Command("/bin/bash", "/tmp/scripts/generate_htpasswd.sh").Output()
	if err != nil {
		fmt.Printf("error %s", err)
	}
	shellScript = bytes.Replace(shellScript, []byte(username), []byte("username"), -1)
	shellScript = bytes.Replace(shellScript, []byte(password), []byte("password"), -1)
	if err = ioutil.WriteFile("/tmp/scripts/generate_htpasswd.sh", shellScript, 0755); err != nil {
		log.Fatal(err)
	}
	htpasswdFile, err := ioutil.ReadFile("/tmp/scripts/htpasswdfile.txt")
	if err != nil {
		log.Fatal(err)
	}

	return htpasswdFile
}
