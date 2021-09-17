package kubernetes

import (
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/prometheus/common/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// NewCertifiedSubscription creates a Certified Subscription
func NewCertifiedSubscription(workshop *workshopv1.Workshop, scheme *runtime.Scheme,
	name string, namespace string, packageName string, channel string, startingCSV string) *olmv1alpha1.Subscription {

	subscription := &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"csc-owner-name":      "certified-operators",
				"csc-owner-namespace": "openshift-marketplace",
			},
		},
		Spec: &olmv1alpha1.SubscriptionSpec{
			Channel:                channel,
			CatalogSource:          "certified-operators",
			CatalogSourceNamespace: "openshift-marketplace",
			StartingCSV:            startingCSV,
			InstallPlanApproval:    olmv1alpha1.ApprovalManual,
			Package:                packageName,
		},
	}

	// Set Workshop instance as the owner and controller
	err := ctrl.SetControllerReference(workshop, subscription, scheme)
	if err != nil {
		log.Error(err, " - Failed to set SetControllerReference for Certified Subscription.")
	}
	return subscription
}

// NewCommunitySubscription creates a Community Subscription
func NewCommunitySubscription(workshop *workshopv1.Workshop, scheme *runtime.Scheme,
	name string, namespace string, packageName string, channel string, startingCSV string) *olmv1alpha1.Subscription {

	subscription := &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"csc-owner-name":      "community-operators",
				"csc-owner-namespace": "openshift-marketplace",
			},
		},
		Spec: &olmv1alpha1.SubscriptionSpec{
			Channel:                channel,
			CatalogSource:          "community-operators",
			CatalogSourceNamespace: "openshift-marketplace",
			StartingCSV:            startingCSV,
			InstallPlanApproval:    olmv1alpha1.ApprovalManual,
			Package:                packageName,
		},
	}

	// Set Workshop instance as the owner and controller
	err := ctrl.SetControllerReference(workshop, subscription, scheme)
	if err != nil {
		log.Error(err, " - Failed to set SetControllerReference for Community Subscription.")
	}
	return subscription
}

// NewRedHatSubscription creates a Red Hat Subscription
func NewRedHatSubscription(workshop *workshopv1.Workshop, scheme *runtime.Scheme,
	name string, namespace string, packageName string, channel string, startingCSV string) *olmv1alpha1.Subscription {

	subscription := &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"csc-owner-name":      "redhat-operators",
				"csc-owner-namespace": "openshift-marketplace",
			},
		},
		Spec: &olmv1alpha1.SubscriptionSpec{
			Channel:                channel,
			CatalogSource:          "redhat-operators",
			CatalogSourceNamespace: "openshift-marketplace",
			StartingCSV:            startingCSV,
			InstallPlanApproval:    olmv1alpha1.ApprovalManual,
			Package:                packageName,
		},
	}

	// Set Workshop instance as the owner and controller
	err := ctrl.SetControllerReference(workshop, subscription, scheme)
	if err != nil {
		log.Error(err, " - Failed to set SetControllerReference for RedHat Subscription.")
	}
	return subscription
}

// NewCustomSubscription creates a Custom Subscription
func NewCustomSubscription(workshop *workshopv1.Workshop, scheme *runtime.Scheme,
	name string, namespace string, packageName string,
	channel string, catalogSource string) *olmv1alpha1.Subscription {

	subscription := &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"csc-owner-name":      "custom-operators",
				"csc-owner-namespace": "openshift-marketplace",
			},
		},
		Spec: &olmv1alpha1.SubscriptionSpec{
			Channel:                channel,
			CatalogSource:          catalogSource,
			CatalogSourceNamespace: "openshift-marketplace",
			Package:                packageName,
		},
	}

	// Set Workshop instance as the owner and controller
	err := ctrl.SetControllerReference(workshop, subscription, scheme)
	if err != nil {
		log.Error(err, " - Failed to set SetControllerReference for Custom Subscription.")
	}
	return subscription
}
