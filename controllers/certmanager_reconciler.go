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

var cermanagerlabels = map[string]string{
	"app.kubernetes.io/part-of": "certmanager",
}

const (
	CERTMANAGERCERTIFIEDSUBSCRIPTIONNAME = "cert-manager-operator"
	CERTMANAGERNAMESPACENAME             = "openshift-operators"
	CERTMANAGERPACKAGENAME               = "cert-manager-operator"
	CERTMANAGERSUBSCRIPTIONNAME          = "cert-manager-operator"
	CERTMANAGERNAME                      = "cert-manager"
	CERTMANAGERCUSTOMRESOURCENAME        = "cert-manager"
)

// Reconciling CertManager
func (r *WorkshopReconciler) reconcileCertManager(workshop *workshopv1.Workshop) (reconcile.Result, error) {
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
	log.Infoln("Creating CertManager")

	channel := workshop.Spec.Infrastructure.CertManager.OperatorHub.Channel
	clusterServiceVersion := workshop.Spec.Infrastructure.CertManager.OperatorHub.ClusterServiceVersion

	CertManagerSubscription := kubernetes.NewCertifiedSubscription(workshop, r.Scheme, CERTMANAGERCERTIFIEDSUBSCRIPTIONNAME, CERTMANAGERNAMESPACENAME,
		CERTMANAGERPACKAGENAME, channel, clusterServiceVersion)
	if err := r.Create(context.TODO(), CertManagerSubscription); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s CertManager Subscription", CertManagerSubscription.Name)
	}

	// Approve the installation
	if err := r.ApproveInstallPlan(clusterServiceVersion, CERTMANAGERSUBSCRIPTIONNAME, CERTMANAGERNAMESPACENAME); err != nil {
		log.Infof("Waiting for CertManager Subscription to create InstallPlan for %s", "CertManageroperator")
		return reconcile.Result{Requeue: true}, nil
	}

	namespace := kubernetes.NewNamespace(workshop, r.Scheme, CERTMANAGERNAME)
	if err := r.Create(context.TODO(), namespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s CertManager Namespace", namespace.Name)
	}

	customresource := certmanager.NewCustomResource(workshop, r.Scheme, CERTMANAGERCUSTOMRESOURCENAME, CERTMANAGERNAME, cermanagerlabels)
	if err := r.Create(context.TODO(), customresource); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Custom Resource", customresource.Name)
	}

	//Success
	return reconcile.Result{}, nil
}

func (r *WorkshopReconciler) deleteCertManager(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	log.Infoln("Deleting CertManager")
	channel := workshop.Spec.Infrastructure.CertManager.OperatorHub.Channel
	clusterServiceVersion := workshop.Spec.Infrastructure.CertManager.OperatorHub.ClusterServiceVersion

	customresource := certmanager.NewCustomResource(workshop, r.Scheme, CERTMANAGERCUSTOMRESOURCENAME, CERTMANAGERNAME, cermanagerlabels)
	// Delete cert-manager resource
	if err := r.Delete(context.TODO(), customresource); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s cert-manager resource", customresource.Name)

	namespace := kubernetes.NewNamespace(workshop, r.Scheme, CERTMANAGERNAME)
	// Delete cert-manager NameSpace
	if err := r.Delete(context.TODO(), namespace); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s cert-manager namespace", namespace.Name)

	CertManagerSubscription := kubernetes.NewCertifiedSubscription(workshop, r.Scheme, CERTMANAGERCERTIFIEDSUBSCRIPTIONNAME, CERTMANAGERNAMESPACENAME,
		CERTMANAGERPACKAGENAME, channel, clusterServiceVersion)
	// Delete certManager Subscription
	if err := r.Delete(context.TODO(), CertManagerSubscription); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s cert-manager Subscription", CertManagerSubscription.Name)
	log.Infoln("Deleted CertManager Successfully")
	//Success
	return reconcile.Result{}, nil
}