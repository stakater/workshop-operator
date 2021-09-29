package controllers

import (
	"context"
	"time"

	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	"github.com/stakater/workshop-operator/common/kubernetes"
	nexus "github.com/stakater/workshop-operator/common/nexus"

	"github.com/stakater/workshop-operator/common/util"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	NEXUSNAMESPACENAME = "nexus"
	NEXUSCRDNAME = "nexus.gpte.opentlc.com"
	NEXUSCRDGROUPNAME= "gpte.opentlc.com"
	NEXUSCRDKINDNAME= "Nexus"
	NEXUSCRDLISTKINDNAME= "NexusList"
	NEXUSCRDPLURALNAME= "nexus"
	NEXUSCRDSINGULARNAME= "nexus"
	NEXUSCRDVERSIONAME= "v1alpha1"
	NEXUSSERVICEACCOUNTNAME= "nexus-operator"
	NEXUSCRNAME= "nexus"
	NEXUSCLUSTERROLENAME = "nexus-operator"
	NEXUSROLEBINDINGSANAME = "nexus-operator"
	NEXUSCLUSTERROLEKINDNAME = "ClusterRole"
	NEXUSANSIBLEDEPLOYMENTNAME = "nexus-operator"
	NEXUSDEPLOYMENTNAME = "nexus"


)


// Reconciling Nexus
func (r *WorkshopReconciler) reconcileNexus(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	enabledNexus := workshop.Spec.Infrastructure.Nexus.Enabled


	if enabledNexus {
		if result, err := r.addNexus(workshop); util.IsRequeued(result, err) {
			return result, err
		}
	}

	return reconcile.Result{}, nil
}

// Add Nexus
func (r *WorkshopReconciler) addNexus(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	imageName := workshop.Spec.Infrastructure.Nexus.Image.Name
	imageTag := workshop.Spec.Infrastructure.Nexus.Image.Tag

	labels := map[string]string{
		"app":                       "nexus",
		"app.kubernetes.io/name":    "nexus",
		"app.kubernetes.io/part-of": "nexus",
	}

	// Create Project
	nexusNamespace := kubernetes.NewNamespace(workshop, r.Scheme, NEXUSNAMESPACENAME)
	if err := r.Create(context.TODO(), nexusNamespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s nexus Project", NEXUSNAMESPACENAME)
	}

	// Create CRD
	nexusCustomResourceDefinition := kubernetes.NewCustomResourceDefinition(workshop, r.Scheme, NEXUSCRDNAME, NEXUSCRDGROUPNAME, NEXUSCRDKINDNAME, NEXUSCRDLISTKINDNAME, NEXUSCRDPLURALNAME, NEXUSCRDSINGULARNAME, NEXUSCRDVERSIONAME, nil, nil)
	if err := r.Create(context.TODO(), nexusCustomResourceDefinition); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s nexus Custom Resource Definition", nexusCustomResourceDefinition.Name)
	}

	// Create Service Account
	nexusServiceAccount := kubernetes.NewServiceAccount(workshop, r.Scheme, NEXUSSERVICEACCOUNTNAME, NEXUSNAMESPACENAME, labels)
	if err := r.Create(context.TODO(), nexusServiceAccount); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s nexus Service Account", nexusServiceAccount.Name)
	}

	// Create Cluster Role
	nexusClusterRole := kubernetes.NewClusterRole(workshop, r.Scheme, NEXUSCLUSTERROLENAME, NEXUSNAMESPACENAME, labels, nexus.NewRules())
	if err := r.Create(context.TODO(), nexusClusterRole); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s nexus Cluster Role", nexusClusterRole.Name)
	}

	// Create Cluster Role Binding
	nexusClusterRoleBinding := kubernetes.NewClusterRoleBindingSA(workshop, r.Scheme, NEXUSROLEBINDINGSANAME, NEXUSNAMESPACENAME, labels, NEXUSSERVICEACCOUNTNAME, NEXUSROLEBINDINGSANAME, NEXUSCLUSTERROLEKINDNAME)
	if err := r.Create(context.TODO(), nexusClusterRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s nexus Cluster Role Binding", nexusClusterRoleBinding.Name)
	}

	// Create Operator
	nexusOperator := kubernetes.NewAnsibleOperatorDeployment(workshop, r.Scheme, NEXUSANSIBLEDEPLOYMENTNAME, NEXUSNAMESPACENAME, labels, imageName+":"+imageTag, NEXUSSERVICEACCOUNTNAME)
	if err := r.Create(context.TODO(), nexusOperator); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s nexus Operator", nexusOperator.Name)
	}

	// Create Custom Resource
	nexusCustomResource := nexus.NewCustomResource(workshop, r.Scheme, NEXUSCRNAME, NEXUSNAMESPACENAME, labels)
	if err := r.Create(context.TODO(), nexusCustomResource); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s nexus Custom Resource", nexusCustomResource.Name)
	}

	// Wait for server to be running
	if !kubernetes.GetK8Client().GetDeploymentStatus(NEXUSDEPLOYMENTNAME, NEXUSNAMESPACENAME) {
		return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, nil
	}

	//Success
	return reconcile.Result{}, nil
}


// Delete Nexus
func (r *WorkshopReconciler) deleteNexus(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	log.Info("Deleting nexus")

	imageName := workshop.Spec.Infrastructure.Nexus.Image.Name
	imageTag := workshop.Spec.Infrastructure.Nexus.Image.Tag

	labels := map[string]string{
		"app":                       "nexus",
		"app.kubernetes.io/name":    "nexus",
		"app.kubernetes.io/part-of": "nexus",
	}

	nexusCustomResource := nexus.NewCustomResource(workshop, r.Scheme, NEXUSCRNAME, NEXUSNAMESPACENAME, labels)
	// Delete Custom Resource
	if err := r.Delete(context.TODO(), nexusCustomResource); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s nexus Custom Resource", nexusCustomResource.Name)

	nexusOperator := kubernetes.NewAnsibleOperatorDeployment(workshop, r.Scheme, NEXUSANSIBLEDEPLOYMENTNAME, NEXUSNAMESPACENAME, labels, imageName+":"+imageTag, NEXUSSERVICEACCOUNTNAME)
	// Delete Operator
	if err := r.Delete(context.TODO(), nexusOperator); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s nexus Operator", nexusOperator.Name)

	nexusClusterRoleBinding := kubernetes.NewClusterRoleBindingSA(workshop, r.Scheme, NEXUSROLEBINDINGSANAME,NEXUSNAMESPACENAME, labels, NEXUSSERVICEACCOUNTNAME, NEXUSROLEBINDINGSANAME, NEXUSCLUSTERROLEKINDNAME)
	// Delete Cluster Role Binding
	if err := r.Delete(context.TODO(), nexusClusterRoleBinding); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s nexus Cluster Role Binding", nexusClusterRoleBinding.Name)

	nexusClusterRole := kubernetes.NewClusterRole(workshop, r.Scheme, NEXUSCLUSTERROLENAME, NEXUSNAMESPACENAME, labels, nexus.NewRules())
	// Delete Cluster Role
	if err := r.Delete(context.TODO(), nexusClusterRole); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s nexus Cluster Role", nexusClusterRole.Name)

	nexusServiceAccount := kubernetes.NewServiceAccount(workshop, r.Scheme, NEXUSSERVICEACCOUNTNAME, NEXUSNAMESPACENAME, labels)
	// Delete Service Account
	if err := r.Delete(context.TODO(), nexusServiceAccount); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s nexus Service Account", nexusServiceAccount.Name)

	nexusCustomResourceDefinition := kubernetes.NewCustomResourceDefinition(workshop, r.Scheme, NEXUSCRDNAME, NEXUSCRDGROUPNAME, NEXUSCRDKINDNAME, NEXUSCRDLISTKINDNAME, NEXUSCRDPLURALNAME, NEXUSCRDSINGULARNAME, NEXUSCRDVERSIONAME, nil, nil)
	// Delete CRD
	if err := r.Delete(context.TODO(), nexusCustomResourceDefinition); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s nexus Custom Resource Definition", nexusCustomResourceDefinition.Name)

	nexusNamespace := kubernetes.NewNamespace(workshop, r.Scheme, NEXUSNAMESPACENAME)
	// Delete Project
	if err := r.Delete(context.TODO(), nexusNamespace); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s  nexus Project", NEXUSNAMESPACENAME)
	log.Info("Nexus deleted successfully")
	//Success
	return reconcile.Result{}, nil
}
