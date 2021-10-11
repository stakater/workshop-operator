package controllers

import (
	"context"
	"fmt"

	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	"github.com/stakater/workshop-operator/common/kubernetes"
	"github.com/stakater/workshop-operator/common/util"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var projectlabels = map[string]string{
	"app.kubernetes.io/part-of": "project",
}

const (
	PROJECT_ROLEBINDING_USER_NAME = "edit"
	PROJECT_SERVICEACCOUNT_NAME   = "default"
	VIEW_CLUSTER_ROLE_NAME        = "view"
	KIND_CLUSTER_ROLE             = "ClusterRole"
)

// Reconciling Project
func (r *WorkshopReconciler) reconcileProject(workshop *workshopv1.Workshop, users int) (reconcile.Result, error) {
	enabledProject := workshop.Spec.Infrastructure.Project.Enabled

	id := 1
	for {
		username := fmt.Sprintf("user%d", id)
		stagingProjectName := fmt.Sprintf("%s%d", workshop.Spec.Infrastructure.Project.StagingName, id)

		if id <= users && enabledProject {
			// Project
			if workshop.Spec.Infrastructure.Project.StagingName != "" {
				if result, err := r.addProject(workshop, stagingProjectName, username); util.IsRequeued(result, err) {
					return result, err
				}
			}

		} else {
			stagingProjectNamespace := kubernetes.NewNamespace(workshop, r.Scheme, stagingProjectName)
			stagingProjectNamespaceFound := &corev1.Namespace{}
			stagingProjectNamespaceErr := r.Get(context.TODO(), types.NamespacedName{Name: stagingProjectNamespace.Name}, stagingProjectNamespaceFound)

			if stagingProjectNamespaceErr != nil && errors.IsNotFound(stagingProjectNamespaceErr) {
				break
			}
		}
		id++
	}

	//Success
	return reconcile.Result{}, nil
}

// Add Project
func (r *WorkshopReconciler) addProject(workshop *workshopv1.Workshop, projectName string, username string) (reconcile.Result, error) {
	log.Infoln("Creating project ")
	projectNamespace := kubernetes.NewNamespace(workshop, r.Scheme, projectName)
	if err := r.Create(context.TODO(), projectNamespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s project Namespace", projectNamespace.Name)
	}

	if result, err := r.manageRoles(workshop, projectNamespace.Name, username); err != nil {
		return result, err
	}

	//Success
	return reconcile.Result{}, nil
}

// Manage Roles
func (r *WorkshopReconciler) manageRoles(workshop *workshopv1.Workshop, projectName string, username string) (reconcile.Result, error) {

	users := []rbac.Subject{}
	userSubject := rbac.Subject{
		Kind: rbac.UserKind,
		Name: username,
	}

	users = append(users, userSubject)

	// User
	userRoleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme, username+"-project", projectName, projectlabels,
		users, PROJECT_ROLEBINDING_USER_NAME, KIND_CLUSTER_ROLE)
	if err := r.Create(context.TODO(), userRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s project Role Binding", userRoleBinding.Name)
	}

	// Default
	defaultRoleBinding := kubernetes.NewRoleBindingSA(workshop, r.Scheme, username+"-default", projectName, projectlabels,
		PROJECT_SERVICEACCOUNT_NAME, VIEW_CLUSTER_ROLE_NAME, KIND_CLUSTER_ROLE)
	if err := r.Create(context.TODO(), defaultRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s project Role Binding", defaultRoleBinding.Name)
	}

	argocdUsers := []rbac.Subject{}
	userSubject = rbac.Subject{
		Kind: rbac.UserKind,
		Name: "system:serviceaccount:argocd:argocd-argocd-application-controller",
	}
	argocdUsers = append(argocdUsers, userSubject)

	//Argo CD
	argocdEditRoleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme,
		username+"-argocd", projectName, projectlabels, argocdUsers, PROJECT_ROLEBINDING_USER_NAME, KIND_CLUSTER_ROLE)
	if err := r.Create(context.TODO(), argocdEditRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s project Role Binding", argocdEditRoleBinding.Name)
	}

	//Success
	return reconcile.Result{}, nil
}

// Delete Manage Roles
func (r *WorkshopReconciler) deleteManageRoles(workshop *workshopv1.Workshop, projectName string, username string) (reconcile.Result, error) {

	users := []rbac.Subject{}
	userSubject := rbac.Subject{
		Kind: rbac.UserKind,
		Name: username,
	}

	users = append(users, userSubject)

	argocdUsers := []rbac.Subject{}
	userSubject = rbac.Subject{
		Kind: rbac.UserKind,
		Name: "system:serviceaccount:argocd:argocd-argocd-application-controller",
	}
	argocdUsers = append(argocdUsers, userSubject)

	argocdEditRoleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme,
		username+"-argocd", projectName, projectlabels, argocdUsers, PROJECT_ROLEBINDING_USER_NAME, KIND_CLUSTER_ROLE)
	// Delete Argo CD
	if err := r.Delete(context.TODO(), argocdEditRoleBinding); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Role Binding", argocdEditRoleBinding.Name)

	defaultRoleBinding := kubernetes.NewRoleBindingSA(workshop, r.Scheme, username+"-default", projectName, projectlabels,
		PROJECT_SERVICEACCOUNT_NAME, VIEW_CLUSTER_ROLE_NAME, KIND_CLUSTER_ROLE)
	// Delete default Role Binding
	if err := r.Delete(context.TODO(), defaultRoleBinding); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s project Role Binding", defaultRoleBinding.Name)

	userRoleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme, username+"-project", projectName, projectlabels,
		users, PROJECT_ROLEBINDING_USER_NAME, KIND_CLUSTER_ROLE)
	// Delete user Role Binding
	if err := r.Delete(context.TODO(), userRoleBinding); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s project Role Binding", userRoleBinding.Name)
	log.Infoln("Deleted project successful")
	//Success
	return reconcile.Result{}, nil
}

// Delete Project
func (r *WorkshopReconciler) deleteProject(workshop *workshopv1.Workshop, users int) (reconcile.Result, error) {
	enabledProject := workshop.Spec.Infrastructure.Project.Enabled
	log.Infoln("Deleting Project ")
	id := 1
	for {
		username := fmt.Sprintf("user%d", id)
		stagingProjectName := fmt.Sprintf("%s%d", workshop.Spec.Infrastructure.Project.StagingName, id)

		if id <= users && enabledProject {
			// Project
			if workshop.Spec.Infrastructure.Project.StagingName != "" {
				if result, err := r.deleteProjectNamespace(workshop, stagingProjectName, username); util.IsRequeued(result, err) {
					return result, err
				}
			}

		} else {
			stagingProjectNamespace := kubernetes.NewNamespace(workshop, r.Scheme, stagingProjectName)
			stagingProjectNamespaceFound := &corev1.Namespace{}
			stagingProjectNamespaceErr := r.Get(context.TODO(), types.NamespacedName{Name: stagingProjectNamespace.Name}, stagingProjectNamespaceFound)

			if stagingProjectNamespaceErr == nil {
				break
			}
		}
		id++
	}

	//Success
	return reconcile.Result{}, nil
}

// Add Project
func (r *WorkshopReconciler) deleteProjectNamespace(workshop *workshopv1.Workshop, projectName string, username string) (reconcile.Result, error) {

	projectNamespace := kubernetes.NewNamespace(workshop, r.Scheme, projectName)
	// Delete a Project
	if err := r.Delete(context.TODO(), projectNamespace); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Project Namespace ", projectNamespace.Name)

	if result, err := r.deleteManageRoles(workshop, projectNamespace.Name, username); err != nil {
		return result, err
	}
	//Success
	return reconcile.Result{}, nil
}
