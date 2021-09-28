package gitea

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewCustomResource returns a new  CustomResource
func NewCustomResource(name string, namespace string, labels map[string]string) *Gitea {
	cr := &Gitea{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: GiteaSpec{
			GiteaVolumeSize:      "4Gi",
			GiteaSsl:             true,
			PostgresqlVolumeSize: "4Gi",
		},
	}
	return cr
}
