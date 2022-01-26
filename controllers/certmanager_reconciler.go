package controllers

import (
	"context"

	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	certmanager "github.com/stakater/workshop-operator/common/certmanager"
	"github.com/stakater/workshop-operator/common/kubernetes"
	"github.com/stakater/workshop-operator/common/util"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	CERT_MANAGER_SUBSCRIPTION_NAME           = "cert-manager-operator"
	CERT_MANAGER_SUBSCRIPTION_NAMESPACE_NAME = "openshift-operators"
	CERT_MANAGER_PACKAGE_NAME                = "cert-manager-operator"
	CERT_MANAGER_NAMESPACE_NAME              = "cert-manager"
	CERT_MANAGER_CUSTOM_RESOURCE_NAME        = "cert-manager"
)

var certManagerLabels = map[string]string{
	"app.kubernetes.io/part-of": "certmanager",
}

// Reconciling CertManager
func (r *WorkshopReconciler) reconcileCertManager(workshop *workshopv1.Workshop, users int) (reconcile.Result, error) {
	enabledCertManager := workshop.Spec.Infrastructure.CertManager.Enabled

	if enabledCertManager {
		if result, err := r.addCertManager(workshop); util.IsRequeued(result, err) {
			return result, err
		}
	}

	//Success
	return reconcile.Result{}, nil
}

func (r *WorkshopReconciler) addCertManager(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.CertManager.OperatorHub.Channel
	clusterServiceVersion := workshop.Spec.Infrastructure.CertManager.OperatorHub.ClusterServiceVersion

	CertManagerSubscription := kubernetes.NewCertifiedSubscription(workshop, r.Scheme, CERT_MANAGER_SUBSCRIPTION_NAME, CERT_MANAGER_SUBSCRIPTION_NAMESPACE_NAME,
		CERT_MANAGER_PACKAGE_NAME, channel, clusterServiceVersion)
	if err := r.Create(context.TODO(), CertManagerSubscription); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Subscription", CertManagerSubscription.Name)
	}

	// Approve the installation
	if err := r.ApproveInstallPlan(clusterServiceVersion, CERT_MANAGER_SUBSCRIPTION_NAME, CERT_MANAGER_SUBSCRIPTION_NAMESPACE_NAME); err != nil {
		log.Infof("Waiting for Subscription to create InstallPlan for %s", CertManagerSubscription.Name)
		return reconcile.Result{Requeue: true}, nil
	}

	namespace := kubernetes.NewNamespace(workshop, r.Scheme, CERT_MANAGER_NAMESPACE_NAME)
	if err := r.Create(context.TODO(), namespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Namespace", namespace.Name)
	}

	customresource := certmanager.NewCustomResource(workshop, r.Scheme, CERT_MANAGER_CUSTOM_RESOURCE_NAME, namespace.Name, certManagerLabels)
	if err := r.Create(context.TODO(), customresource); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Custom Resource", customresource.Name)
	}

	//Success
	return reconcile.Result{}, nil
}

func (r *WorkshopReconciler) deleteCertManager(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.CertManager.OperatorHub.Channel
	clusterServiceVersion := workshop.Spec.Infrastructure.CertManager.OperatorHub.ClusterServiceVersion

	namespace := kubernetes.NewNamespace(workshop, r.Scheme, CERT_MANAGER_NAMESPACE_NAME)

	customresource := certmanager.NewCustomResource(workshop, r.Scheme, CERT_MANAGER_CUSTOM_RESOURCE_NAME, namespace.Name, certManagerLabels)
	// Delete Custom Resource
	if err := r.Delete(context.TODO(), customresource); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Custom Resource", customresource.Name)

	// Delete Namespace
	if err := r.Delete(context.TODO(), namespace); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s  Namespace", namespace.Name)

	CertManagerSubscription := kubernetes.NewCertifiedSubscription(workshop, r.Scheme, CERT_MANAGER_SUBSCRIPTION_NAME, CERT_MANAGER_SUBSCRIPTION_NAMESPACE_NAME,
		CERT_MANAGER_PACKAGE_NAME, channel, clusterServiceVersion)
	// Delete certManager Subscription
	if err := r.Delete(context.TODO(), CertManagerSubscription); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Subscription", CertManagerSubscription.Name)

	//Success
	return reconcile.Result{}, nil
}
