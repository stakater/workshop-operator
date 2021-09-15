package controllers

import (
	"context"
	"fmt"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

	securityv1 "github.com/openshift/api/security/v1"
	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	"github.com/stakater/workshop-operator/common/kubernetes"
	"github.com/stakater/workshop-operator/common/util"

	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconciling IstioWorkspace
func (r *WorkshopReconciler) reconcileIstioWorkspace(workshop *workshopv1.Workshop, users int) (reconcile.Result, error) {
	enabled := workshop.Spec.Infrastructure.IstioWorkspace.Enabled

	if enabled {

		if result, err := r.addIstioWorkspace(workshop, users); util.IsRequeued(result, err) {
			return result, err
		}
	}
	if enabled {

		if result, err := r.deleteIstioWorkspace(workshop, users); util.IsRequeued(result, err) {
			return result, err
		}
	}

	//Success
	return reconcile.Result{}, nil
}

func (r *WorkshopReconciler) addIstioWorkspace(workshop *workshopv1.Workshop, users int) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.IstioWorkspace.OperatorHub.Channel
	clusterserviceversion := workshop.Spec.Infrastructure.IstioWorkspace.OperatorHub.ClusterServiceVersion

	subscription := kubernetes.NewCommunitySubscription(workshop, r.Scheme, "istio-workspace-operator", "openshift-operators",
		"istio-workspace-operator", channel, clusterserviceversion)
	if err := r.Create(context.TODO(), subscription); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Subscription", subscription.Name)
	}

	if err := r.ApproveInstallPlan(clusterserviceversion, "istio-workspace-operator", "openshift-operators"); err != nil {
		log.Infof("Waiting for Subscription to create InstallPlan for %s", subscription.Name)
		return reconcile.Result{Requeue: true}, nil
	}

	labels := map[string]string{
		"app.kubernetes.io/part-of": "istio-workspace",
	}

	for id := 1; id <= users; id++ {
		username := fmt.Sprintf("user%d", id)
		stagingProjectName := fmt.Sprintf("%s%d", workshop.Spec.Infrastructure.Project.StagingName, id)

		role := kubernetes.NewRole(workshop, r.Scheme,
			username+"-istio-workspace", stagingProjectName, labels, kubernetes.IstioWorkspaceUserRules())
		if err := r.Create(context.TODO(), role); err != nil && !errors.IsAlreadyExists(err) {
			return reconcile.Result{}, err
		} else if err == nil {
			log.Infof("Created %s Role", role.Name)
		}

		users := []rbac.Subject{
			{
				Kind: rbac.UserKind,
				Name: username,
			},
		}

		roleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme,
			username+"-istio-workspace", stagingProjectName, labels, users, username+"-istio-workspace", "Role")
		if err := r.Create(context.TODO(), roleBinding); err != nil && !errors.IsAlreadyExists(err) {
			return reconcile.Result{}, err
		} else if err == nil {
			log.Infof("Created %s Role Binding", roleBinding.Name)
		}

		// Create SCC
		serviceAccountUser := "system:serviceaccount:" + stagingProjectName + ":default"

		privilegedSCCFound := &securityv1.SecurityContextConstraints{}
		if err := r.Get(context.TODO(), types.NamespacedName{Name: "privileged"}, privilegedSCCFound); err != nil {
			return reconcile.Result{}, err
		}

		if !util.StringInSlice(serviceAccountUser, privilegedSCCFound.Users) {
			privilegedSCCFound.Users = append(privilegedSCCFound.Users, serviceAccountUser)
			if err := r.Update(context.TODO(), privilegedSCCFound); err != nil {
				return reconcile.Result{}, err
			} else if err == nil {
				log.Infof("Updated %s SCC", privilegedSCCFound.Name)
			}
		}

		anyuidSCCFound := &securityv1.SecurityContextConstraints{}
		if err := r.Get(context.TODO(), types.NamespacedName{Name: "anyuid"}, anyuidSCCFound); err != nil {
			return reconcile.Result{}, err
		}

		if !util.StringInSlice(serviceAccountUser, anyuidSCCFound.Users) {
			anyuidSCCFound.Users = append(anyuidSCCFound.Users, serviceAccountUser)
			if err := r.Update(context.TODO(), anyuidSCCFound); err != nil {
				return reconcile.Result{}, err
			} else if err == nil {
				log.Infof("Updated %s SCC", anyuidSCCFound.Name)
			}
		}
	}

	//Success
	return reconcile.Result{}, nil
}

// delete IstioWorkspace
func (r *WorkshopReconciler) deleteIstioWorkspace(workshop *workshopv1.Workshop, users int) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.IstioWorkspace.OperatorHub.Channel
	clusterserviceversion := workshop.Spec.Infrastructure.IstioWorkspace.OperatorHub.ClusterServiceVersion

	labels := map[string]string{
		"app.kubernetes.io/part-of": "istio-workspace",
	}

	for id := 1; id <= users; id++ {
		username := fmt.Sprintf("user%d", id)
		stagingProjectName := fmt.Sprintf("%s%d", workshop.Spec.Infrastructure.Project.StagingName, id)
		users := []rbac.Subject{
			{
				Kind: rbac.UserKind,
				Name: username,
			},
		}

		role := kubernetes.NewRole(workshop, r.Scheme,
			username+"-istio-workspace", stagingProjectName, labels, kubernetes.IstioWorkspaceUserRules())
		roleFound := &rbac.Role{}
		roleErr := r.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: role.Namespace}, roleFound)
		if roleErr == nil {
			// Delete role
			if err := r.Delete(context.TODO(), role); err != nil {
				return reconcile.Result{}, err
			}
			log.Infof("Deleted %s Role", role.Name)
		}

		roleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme,
			username+"-istio-workspace", stagingProjectName, labels, users, username+"-istio-workspace", "Role")
		roleBindingFound := &rbac.RoleBinding{}
		roleBindingErr := r.Get(context.TODO(), types.NamespacedName{Name: roleBindingFound.Name, Namespace: roleBindingFound.Namespace}, roleBindingFound)
		if roleBindingErr == nil {
			// Delete RoleBinding
			if err := r.Delete(context.TODO(), roleBinding); err != nil {
				return reconcile.Result{}, err
			}
			log.Infof("Deleted %s Role Binding", roleBinding.Name)
		}
	}

	subscription := kubernetes.NewCommunitySubscription(workshop, r.Scheme, "istio-workspace-operator", "openshift-operators",
		"istio-workspace-operator", channel, clusterserviceversion)
	subscriptionFound := &olmv1alpha1.Subscription{}
	subscriptionErr := r.Get(context.TODO(), types.NamespacedName{Name: subscription.Name, Namespace: subscription.Namespace}, subscriptionFound)
	if subscriptionErr == nil {
		// Delete Subscription
		if err := r.Delete(context.TODO(), subscription); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Subscription", subscription.Name)
	}
	//Success
	return reconcile.Result{}, nil
}
