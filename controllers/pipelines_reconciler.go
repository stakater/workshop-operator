package controllers

import (
	"context"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	"github.com/stakater/workshop-operator/common/kubernetes"

	"github.com/stakater/workshop-operator/common/util"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconciling Pipeline
func (r *WorkshopReconciler) reconcilePipelines(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	enabledPipeline := workshop.Spec.Infrastructure.Pipeline.Enabled

	if enabledPipeline {
		if result, err := r.addPipelines(workshop); util.IsRequeued(result, err) {
			return result, err
		}
	}
	if enabledPipeline {
		if result, err := r.deletePipelines(workshop); util.IsRequeued(result, err) {
			return result, err
		}
	}

	//Success
	return reconcile.Result{}, nil
}

// Add Pipelines
func (r *WorkshopReconciler) addPipelines(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	name := "openshift-pipelines-operator-rh"
	channel := workshop.Spec.Infrastructure.Pipeline.OperatorHub.Channel
	clusterServiceVersion := workshop.Spec.Infrastructure.Pipeline.OperatorHub.ClusterServiceVersion

	// Create Subscription
	pipelineSubscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, name, "openshift-operators",
		name, channel, clusterServiceVersion)
	if err := r.Create(context.TODO(), pipelineSubscription); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Subscription", pipelineSubscription.Name)
	}

	// Approve the installation
	if err := r.ApproveInstallPlan(clusterServiceVersion, name, "openshift-operators"); err != nil {
		log.Infof("Waiting for Subscription to create InstallPlan for %s", name)
		return reconcile.Result{Requeue: true}, nil
	}

	//Success
	return reconcile.Result{}, nil
}

// delete Pipelines
func (r *WorkshopReconciler) deletePipelines(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	name := "openshift-pipelines-operator-rh"
	channel := workshop.Spec.Infrastructure.Pipeline.OperatorHub.Channel
	clusterServiceVersion := workshop.Spec.Infrastructure.Pipeline.OperatorHub.ClusterServiceVersion

	// Create Subscription
	pipelineSubscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, name, "openshift-operators",
		name, channel, clusterServiceVersion)
	pipelineSubscriptionFound := &olmv1alpha1.Subscription{}
	pipelineSubscriptionErr := r.Get(context.TODO(), types.NamespacedName{Name: pipelineSubscription.Name, Namespace: pipelineSubscription.Namespace}, pipelineSubscriptionFound)
	if pipelineSubscriptionErr == nil {
		// Delete Subscription
		if err := r.Delete(context.TODO(), pipelineSubscription); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Subscription", pipelineSubscription.Name)
	}

	//Success
	return reconcile.Result{}, nil
}
