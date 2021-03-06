package controllers

import (
	"context"
	"errors"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/prometheus/common/log"
	"github.com/stakater/workshop-operator/common/kubernetes"
	"github.com/stakater/workshop-operator/common/util"
)

// ApproveInstallPlan approves manually the install of a specific CSV
func (r *WorkshopReconciler) ApproveInstallPlan(clusterServiceVersion string, subscriptionName string, namespace string) error {

	subscription := &olmv1alpha1.Subscription{}
	if err := kubernetes.GetObject(r, subscriptionName, namespace, subscription); err != nil {
		return err
	}

	if (clusterServiceVersion == "" && subscription.Status.InstalledCSV == "") ||
		(clusterServiceVersion != "" && (subscription.Status.InstalledCSV != clusterServiceVersion)) {
		if subscription.Status.InstallPlanRef == nil {
			return errors.New("InstallPlan Approval: Subscription is not ready yet")
		}

		installPlan := &olmv1alpha1.InstallPlan{}
		if err := kubernetes.GetObject(r, subscription.Status.InstallPlanRef.Name, namespace, installPlan); err != nil {
			return err
		}

		if util.StringInSlice(clusterServiceVersion, installPlan.Spec.ClusterServiceVersionNames) && !installPlan.Spec.Approved {
			installPlan.Spec.Approved = true
			if err := r.Update(context.TODO(), installPlan); err != nil {
				return err
			}
			log.Infof("%s InstallPlan in %s project Approved", installPlan.Name, namespace)
		}
	}
	return nil
}
