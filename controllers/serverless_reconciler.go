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

const (
	SERVERLESSNAMESPACENAME ="openshift-serverless"
	SERVERLESSSUBSCRIPTIONNAME = "serverless-operator"
	SERVERLESSPACKAGENAME ="serverless-operator"
	KNATIVESERVINGNAMESPACENAME = "knative-serving"
	KNATIVEEVENTINGNAMESPACENAME = "knative-eventing"

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

// Add Serverless
func (r *WorkshopReconciler) addServerless(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	log.Infoln("Creating Serverless ")

	channel := workshop.Spec.Infrastructure.Serverless.OperatorHub.Channel
	clusterServiceVersion := workshop.Spec.Infrastructure.Serverless.OperatorHub.ClusterServiceVersion

	namespace := kubernetes.NewNamespace(workshop, r.Scheme, SERVERLESSNAMESPACENAME)
	if err := r.Create(context.TODO(), namespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Serverless Project", namespace.Name)
	}

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, SERVERLESSSUBSCRIPTIONNAME, namespace.Name, SERVERLESSPACKAGENAME,
		channel, clusterServiceVersion)
	if err := r.Create(context.TODO(), subscription); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Serverless Subscription", subscription.Name)
	}

	knativeServingNamespace := kubernetes.NewNamespace(workshop, r.Scheme, KNATIVESERVINGNAMESPACENAME)
	if err := r.Create(context.TODO(), knativeServingNamespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Namespace", knativeServingNamespace.Name)
	}

	knativeEventingNamespace := kubernetes.NewNamespace(workshop, r.Scheme, KNATIVEEVENTINGNAMESPACENAME)
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
	log.Infoln("Deleting Serverless ")
	channel := workshop.Spec.Infrastructure.Serverless.OperatorHub.Channel
	clusterServiceVersion := workshop.Spec.Infrastructure.Serverless.OperatorHub.ClusterServiceVersion

	namespace := kubernetes.NewNamespace(workshop, r.Scheme, SERVERLESSNAMESPACENAME)
	knativeServingNamespace := kubernetes.NewNamespace(workshop, r.Scheme, KNATIVESERVINGNAMESPACENAME)

	knativeEventingNamespace := kubernetes.NewNamespace(workshop, r.Scheme, KNATIVEEVENTINGNAMESPACENAME)
	//Delete knativeEventing Namespace
	if err := r.Delete(context.TODO(), knativeEventingNamespace); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Serverless Namespace", knativeEventingNamespace.Name)

	//Delete knativeServing Namespace
	if err := r.Delete(context.TODO(), knativeServingNamespace); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Serverless Namespace", knativeServingNamespace.Name)

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, SERVERLESSSUBSCRIPTIONNAME, namespace.Name, SERVERLESSPACKAGENAME,
		channel, clusterServiceVersion)
	//Delete subscription
	if err := r.Delete(context.TODO(), subscription); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Serverless Subscription", subscription.Name)

	// Delete namespace
	if err := r.Delete(context.TODO(), namespace); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s  Serverless namespace", namespace.Name)
	log.Infoln("Deleted Serverless Successfully")
	// TODO
	// Add  knativeServingNamespace to ServiceMeshMember
	// Delete CR

	//Success
	return reconcile.Result{}, nil
}

