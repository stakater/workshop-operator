package user

import (
	user1 "github.com/openshift/api/user/v1"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// NewUser creates a User
func NewUser(workshop *workshopv1.Workshop, scheme *runtime.Scheme, username string) *user1.User {

	user := &user1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: username,
		},
		FullName: username,
	}
	return user
}
