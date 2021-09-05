package controllers

import (
	"context"

	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	"github.com/stakater/workshop-operator/common/kubernetes"
	nexus "github.com/stakater/workshop-operator/common/nexus"

	"github.com/stakater/workshop-operator/common/util"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconciling Nexus
func (r *WorkshopReconciler) reconcileNexus(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	enabledNexus := workshop.Spec.Infrastructure.Nexus.Enabled

	nexusNamespaceName := "nexus"

	if enabledNexus {
		if result, err := r.addNexus(workshop, nexusNamespaceName); util.IsRequeued(result, err) {
			return result, err
		}
	}

	//Success
	return reconcile.Result{}, nil
}

// Add Nexus
func (r *WorkshopReconciler) addNexus(workshop *workshopv1.Workshop, nexusNamespaceName string) (reconcile.Result, error) {

	labels := map[string]string{
		"app":                       "nexus",
		"app.kubernetes.io/name":    "nexus",
		"app.kubernetes.io/part-of": "nexus",
	}

	// Create Project
	nexusNamespace := kubernetes.NewNamespace(workshop, r.Scheme, nexusNamespaceName)
	if err := r.Create(context.TODO(), nexusNamespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Project", nexusNamespace.Name)
	}

	// Create CRD
	nexusCustomResourceDefinition := kubernetes.NewCustomResourceDefinition(workshop, r.Scheme, "nexus.gpte.opentlc.com", "gpte.opentlc.com", "Nexus", "NexusList", "nexus", "nexus", "v1alpha1", nil, nil)
	if err := r.Create(context.TODO(), nexusCustomResourceDefinition); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Custom Resource Definition", nexusCustomResourceDefinition.Name)
	}

	// Create Service Account
	nexusServiceAccount := kubernetes.NewServiceAccount(workshop, r.Scheme, "nexus-operator", nexusNamespace.Name, labels)
	if err := r.Create(context.TODO(), nexusServiceAccount); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Service Account", nexusServiceAccount.Name)
	}

	// Create Cluster Role
	nexusClusterRole := kubernetes.NewClusterRole(workshop, r.Scheme, "nexus-operator", nexusNamespace.Name, labels, nexus.NewRules())
	if err := r.Create(context.TODO(), nexusClusterRole); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Cluster Role", nexusClusterRole.Name)
	}

	// Create Cluster Role Binding
	nexusClusterRoleBinding := kubernetes.NewClusterRoleBindingSA(workshop, r.Scheme, "nexus-operator", nexusNamespace.Name, labels, "nexus-operator", "nexus-operator", "ClusterRole")
	if err := r.Create(context.TODO(), nexusClusterRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Cluster Role Binding", nexusClusterRoleBinding.Name)
	}

	// Create Operator
	nexusOperator := kubernetes.NewAnsibleOperatorDeployment(workshop, r.Scheme, "nexus-operator", nexusNamespace.Name, labels, "quay.io/stakater/nexus-operator:v0.10", "nexus-operator")
	if err := r.Create(context.TODO(), nexusOperator); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Operator", nexusOperator.Name)
	}

	// Create Custom Resource
	nexusCustomResource := nexus.NewCustomResource(workshop, r.Scheme, "nexus", nexusNamespace.Name, labels)
	if err := r.Create(context.TODO(), nexusCustomResource); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Custom Resource", nexusCustomResource.Name)
	}

	//Success
	return reconcile.Result{}, nil
}

// TODO: Delete Nexus
// TODO: Delete namespace
