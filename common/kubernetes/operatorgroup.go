package kubernetes

import (
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// NewOperatorGroup creates an Operator Group
func NewOperatorGroup(workshop *workshopv1.Workshop, scheme *runtime.Scheme,
	name string, namespace string) *olmv1.OperatorGroup {

	operatorgroup := &olmv1.OperatorGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: olmv1.OperatorGroupSpec{
			TargetNamespaces: []string{
				namespace,
			},
		},
	}
	return operatorgroup
}
