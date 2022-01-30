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

// Reconciling Serverless
func (r *WorkshopReconciler) reconcileServerless(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	enabledServerless := workshop.Spec.Infrastructure.Serverless.Enabled

	if enabledServerless {

		if result, err := r.addServerless(workshop); util.IsRequeued(result, err) {
			return result, err
		}
	}

	//Success
	return reconcile.Result{}, nil
}

const (
	SERVERLESS_NAMESPACE_NAME              = "openshift-serverless"
	SERVERLESS_SUBSCRIPTION_NAME           = "serverless-operator"
	SERVERLESS_SUBSCRIPTION_NAMESPACE_NAME = "openshift-serverless"
	SERVERLESS_PACKAGE_NAME                = "serverless-operator"
	KNATIVE_SERVING_NAMESPACE_NAME         = "knative-serving"
	KNATIVE_EVENTING_NAMESPACE_NAME        = "knative-eventing"
)

// Add Serverless
func (r *WorkshopReconciler) addServerless(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	log.Info("start addServerless method")
	channel := workshop.Spec.Infrastructure.Serverless.OperatorHub.Channel
	clusterServiceVersion := workshop.Spec.Infrastructure.Serverless.OperatorHub.ClusterServiceVersion

	// Create Serverless Namespace
	namespace := kubernetes.NewNamespace(workshop, r.Scheme, SERVERLESS_NAMESPACE_NAME)
	if err := r.Create(context.TODO(), namespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Project", namespace.Name)
	}

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, SERVERLESS_SUBSCRIPTION_NAME, SERVERLESS_SUBSCRIPTION_NAMESPACE_NAME, SERVERLESS_PACKAGE_NAME,
		channel, clusterServiceVersion)
	if err := r.Create(context.TODO(), subscription); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Subscription", subscription.Name)
	}

	knativeServingNamespace := kubernetes.NewNamespace(workshop, r.Scheme, KNATIVE_SERVING_NAMESPACE_NAME)
	if err := r.Create(context.TODO(), knativeServingNamespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Namespace", knativeServingNamespace.Name)
	}

	knativeEventingNamespace := kubernetes.NewNamespace(workshop, r.Scheme, KNATIVE_EVENTING_NAMESPACE_NAME)
	if err := r.Create(context.TODO(), knativeEventingNamespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Namespace", knativeEventingNamespace.Name)
	}

	// TODO
	// Add  knativeServingNamespace to ServiceMeshMember
	// Create CR

	//Success
	return reconcile.Result{}, nil
}

// delete Serverless
func (r *WorkshopReconciler) deleteServerless(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.Serverless.OperatorHub.Channel
	clusterServiceVersion := workshop.Spec.Infrastructure.Serverless.OperatorHub.ClusterServiceVersion

	namespace := kubernetes.NewNamespace(workshop, r.Scheme, SERVERLESS_NAMESPACE_NAME)
	knativeServingNamespace := kubernetes.NewNamespace(workshop, r.Scheme, KNATIVE_SERVING_NAMESPACE_NAME)
	knativeEventingNamespace := kubernetes.NewNamespace(workshop, r.Scheme, KNATIVE_EVENTING_NAMESPACE_NAME)

	//Delete knativeEventing Namespace
	if err := r.Delete(context.TODO(), knativeEventingNamespace); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Namespace", knativeEventingNamespace.Name)

	//Delete knativeServing Namespace
	if err := r.Delete(context.TODO(), knativeServingNamespace); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Namespace", knativeServingNamespace.Name)

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, SERVERLESS_SUBSCRIPTION_NAME, SERVERLESS_SUBSCRIPTION_NAMESPACE_NAME, SERVERLESS_PACKAGE_NAME,
		channel, clusterServiceVersion)

	//Delete subscription
	if err := r.Delete(context.TODO(), subscription); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Subscription", subscription.Name)

	// Delete namespace
	if err := r.Delete(context.TODO(), namespace); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s namespace", namespace.Name)

	//

	// TODO
	// Add  knativeServingNamespace to ServiceMeshMember
	// Delete CR

	//Success
	return reconcile.Result{}, nil
}
