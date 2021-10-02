package controllers

import (
	"context"
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

	//Success
	return reconcile.Result{}, nil
}

const (
	PIPELINESSUBSCRIPTIONNAME = "openshift-pipelines-operator-rh"
	PIPELINESNAMESPACENAME    = "openshift-operators"
	PIPELINESPACKAGENAMENAME  = "openshift-pipelines-operator-rh"
)

// Add Pipelines
func (r *WorkshopReconciler) addPipelines(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	log.Info("Creating Pipelines")

	channel := workshop.Spec.Infrastructure.Pipeline.OperatorHub.Channel
	clusterServiceVersion := workshop.Spec.Infrastructure.Pipeline.OperatorHub.ClusterServiceVersion

	// Create Subscription
	pipelineSubscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, PIPELINESSUBSCRIPTIONNAME, PIPELINESNAMESPACENAME,
		PIPELINESPACKAGENAMENAME, channel, clusterServiceVersion)
	if err := r.Create(context.TODO(), pipelineSubscription); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Subscription", pipelineSubscription.Name)
	}

	// Approve the installation
	if err := r.ApproveInstallPlan(clusterServiceVersion, PIPELINESSUBSCRIPTIONNAME, PIPELINESNAMESPACENAME); err != nil {
		log.Infof("Waiting for Subscription to create InstallPlan for %s", PIPELINESSUBSCRIPTIONNAME)
		return reconcile.Result{Requeue: true}, nil
	}

	//Success
	return reconcile.Result{}, nil
}

// delete Pipelines
func (r *WorkshopReconciler) deletePipelines(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	log.Info("Deleting Pipelines")
	channel := workshop.Spec.Infrastructure.Pipeline.OperatorHub.Channel
	clusterServiceVersion := workshop.Spec.Infrastructure.Pipeline.OperatorHub.ClusterServiceVersion

	// Create Subscription
	pipelineSubscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, PIPELINESSUBSCRIPTIONNAME, PIPELINESNAMESPACENAME,
		PIPELINESPACKAGENAMENAME, channel, clusterServiceVersion)
	// Delete Subscription
	if err := r.Delete(context.TODO(), pipelineSubscription); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Subscription", pipelineSubscription.Name)
	log.Info("Pipelines deleted successfully")
	//Success
	return reconcile.Result{}, nil
}
