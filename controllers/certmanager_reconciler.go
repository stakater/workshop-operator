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

// Reconciling CertManager
func (r *WorkshopReconciler) reconcileCertManager(workshop *workshopv1.Workshop, users int) (reconcile.Result, error) {
	enabledCertManager := workshop.Spec.Infrastructure.CertManager.Enabled

	if enabledCertManager {
		if result, err := r.addCertManager(workshop, users); util.IsRequeued(result, err) {
			return result, err
		}
	}

	//Success
	return reconcile.Result{}, nil
}

func (r *WorkshopReconciler) addCertManager(workshop *workshopv1.Workshop, users int) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.CertManager.OperatorHub.Channel
	clusterServiceVersion := workshop.Spec.Infrastructure.CertManager.OperatorHub.ClusterServiceVersion

	CertManagerSubscription := kubernetes.NewCertifiedSubscription(workshop, r.Scheme, "cert-manager-operator", "openshift-operators",
		"cert-manager-operator", channel, clusterServiceVersion)
	if err := r.Create(context.TODO(), CertManagerSubscription); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Subscription", CertManagerSubscription.Name)
	}

	// Approve the installation
	if err := r.ApproveInstallPlan(clusterServiceVersion, "cert-manager-operator", "openshift-operators"); err != nil {
		log.Infof("Waiting for Subscription to create InstallPlan for %s", "CertManageroperator")
		return reconcile.Result{Requeue: true}, nil
	}

	namespace := kubernetes.NewNamespace(workshop, r.Scheme, "cert-manager")
	if err := r.Create(context.TODO(), namespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Namespace", namespace.Name)
	}

	labels := map[string]string{
		"app.kubernetes.io/part-of": "certmanager",
	}

	customresource := certmanager.NewCustomResource(workshop, r.Scheme, "cert-manager", namespace.Name, labels)
	if err := r.Create(context.TODO(), customresource); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Custom Resource", customresource.Name)
	}

	//Success
	return reconcile.Result{}, nil
}

/**
func (r *WorkshopReconciler) deleteCertManager(workshop *workshopv1.Workshop, users int) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.CertManager.OperatorHub.Channel
	clusterServiceVersion := workshop.Spec.Infrastructure.CertManager.OperatorHub.ClusterServiceVersion
	labels := map[string]string{
		"app.kubernetes.io/part-of": "certmanager",
	}
	namespace := kubernetes.NewNamespace(workshop, r.Scheme, "cert-manager")

	customresource := certmanager.NewCustomResource(workshop, r.Scheme, "cert-manager", namespace.Name, labels)
	certmanagerresourceFound := &certmanager.CertManager{}
	certmanagerresourceErr := r.Get(context.TODO(), types.NamespacedName{Name: customresource.Name, Namespace: namespace.Name}, certmanagerresourceFound)
	if certmanagerresourceErr == nil {
		// Delete cert-manager resource
		if err := r.Delete(context.TODO(), customresource); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s cert-manager resource", customresource.Name)
	}

	certmanagerNameSpaceFound := &corev1.Namespace{}
	certmanagerNameSpaceErr := r.Get(context.TODO(), types.NamespacedName{Name: namespace.Name}, certmanagerNameSpaceFound)
	if certmanagerNameSpaceErr == nil {
		// Delete cert-manager NameSpace
		if err := r.Delete(context.TODO(), namespace); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s cert-manager namespace", namespace.Name)
	}
	CertManagerSubscription := kubernetes.NewCertifiedSubscription(workshop, r.Scheme, "cert-manager-operator", "openshift-operators",
		"cert-manager-operator", channel, clusterServiceVersion)
	certManagerSubscriptionFund := &olmv1alpha1.Subscription{}
	certManagerSubscriptionErr := r.Get(context.TODO(), types.NamespacedName{Name: CertManagerSubscription.Name, Namespace: namespace.Name}, certManagerSubscriptionFund)
	if certManagerSubscriptionErr == nil {
		// Delete certManager Subscription
		if err := r.Delete(context.TODO(), CertManagerSubscription); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s cert-manager Subscription", CertManagerSubscription.Name)
	}
	//Success
	return reconcile.Result{}, nil
}
**/
