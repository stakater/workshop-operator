package kubernetes

import (
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// NewCustomResourceDefinition returns a Custom Resource Definition (CRD)
func NewCustomResourceDefinition(workshop *workshopv1.Workshop, scheme *runtime.Scheme,
	name string, group string, kind string, listKind string, plural string, singular string, version string, shortNames []string, additionalPrinterColumns []apiextensionsv1beta1.CustomResourceColumnDefinition) *apiextensionsv1beta1.CustomResourceDefinition {

	crd := &apiextensionsv1beta1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
			Group:   group,
			Version: version,
			Scope:   apiextensionsv1beta1.NamespaceScoped,
			Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
				Kind:       kind,
				ListKind:   listKind,
				Plural:     plural,
				Singular:   singular,
				ShortNames: shortNames,
			},
			Subresources: &apiextensionsv1beta1.CustomResourceSubresources{
				Status: &apiextensionsv1beta1.CustomResourceSubresourceStatus{},
			},
			AdditionalPrinterColumns: additionalPrinterColumns,
		},
	}

	return crd
}
