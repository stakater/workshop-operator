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

			if !(stagingProjectNamespaceErr != nil && errors.IsNotFound(stagingProjectNamespaceErr)) {
				if result, err := r.deleteProject(stagingProjectNamespace); util.IsRequeued(result, err) {
					return result, err
				}
			}
		}

		id++
	}

	//Success
	return reconcile.Result{}, nil
}

// Add Project
func (r *WorkshopReconciler) addProject(workshop *workshopv1.Workshop, projectName string, username string) (reconcile.Result, error) {

	projectNamespace := kubernetes.NewNamespace(workshop, r.Scheme, projectName)
	if err := r.Create(context.TODO(), projectNamespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Namespace", projectNamespace.Name)
	}

	if result, err := r.manageRoles(workshop, projectNamespace.Name, username); err != nil {
		return result, err
	}
	if result, err := r.deletemanageRoles(workshop, projectNamespace.Name, username); err != nil {
		return result, err
	}
	//Success
	return reconcile.Result{}, nil
}

// Delete Project
func (r *WorkshopReconciler) deleteProject(namespaces *corev1.Namespace) (reconcile.Result, error) {

	if err := r.Delete(context.TODO(), namespaces); err != nil && !errors.IsNotFound(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Deleted %s Namespace", namespaces.Name)
	}

	//Success
	return reconcile.Result{}, nil
}

// Manage Roles
func (r *WorkshopReconciler) manageRoles(workshop *workshopv1.Workshop, projectName string, username string) (reconcile.Result, error) {

	labels := map[string]string{
		"app.kubernetes.io/part-of": "project",
	}

	users := []rbac.Subject{}
	userSubject := rbac.Subject{
		Kind: rbac.UserKind,
		Name: username,
	}

	users = append(users, userSubject)

	// User
	userRoleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme, username+"-project", projectName, labels,
		users, "edit", "ClusterRole")
	if err := r.Create(context.TODO(), userRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Role Binding", userRoleBinding.Name)
	}

	// Default
	defaultRoleBinding := kubernetes.NewRoleBindingSA(workshop, r.Scheme, username+"-default", projectName, labels,
		"default", "view", "ClusterRole")
	if err := r.Create(context.TODO(), defaultRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Role Binding", defaultRoleBinding.Name)
	}

	argocdUsers := []rbac.Subject{}
	userSubject = rbac.Subject{
		Kind: rbac.UserKind,
		Name: "system:serviceaccount:argocd:argocd-argocd-application-controller",
	}
	argocdUsers = append(argocdUsers, userSubject)

	//Argo CD
	argocdEditRoleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme,
		username+"-argocd", projectName, labels, argocdUsers, "edit", "ClusterRole")
	if err := r.Create(context.TODO(), argocdEditRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Role Binding", argocdEditRoleBinding.Name)
	}

	//Success
	return reconcile.Result{}, nil
}

// Delete Manage Roles
func (r *WorkshopReconciler) deletemanageRoles(workshop *workshopv1.Workshop, projectName string, username string) (reconcile.Result, error) {

	labels := map[string]string{
		"app.kubernetes.io/part-of": "project",
	}

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
		username+"-argocd", projectName, labels, argocdUsers, "edit", "ClusterRole")
	argocdEditRoleBindingFound := &rbac.RoleBinding{}
	argocdEditRoleBindingErr := r.Get(context.TODO(), types.NamespacedName{Name: argocdEditRoleBinding.Name, Namespace: projectName}, argocdEditRoleBindingFound)
	if argocdEditRoleBindingErr == nil {
		// Delete Argo CD
		if err := r.Delete(context.TODO(), argocdEditRoleBinding); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Role Binding", argocdEditRoleBinding.Name)
	}

	defaultRoleBinding := kubernetes.NewRoleBindingSA(workshop, r.Scheme, username+"-default", projectName, labels,
		"default", "view", "ClusterRole")
	defaultRoleBindingFound := &rbac.RoleBinding{}
	defaultRoleBindingErr := r.Get(context.TODO(), types.NamespacedName{Name: defaultRoleBinding.Name, Namespace: projectName}, defaultRoleBindingFound)
	if defaultRoleBindingErr == nil {
		// Delete default Role Binding
		if err := r.Delete(context.TODO(), defaultRoleBinding); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Role Binding", defaultRoleBinding.Name)
	}

	userRoleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme, username+"-project", projectName, labels,
		users, "edit", "ClusterRole")
	userRoleBindingFound := &rbac.RoleBinding{}
	userRoleBindingErr := r.Get(context.TODO(), types.NamespacedName{Name: userRoleBinding.Name, Namespace: projectName}, userRoleBindingFound)
	if userRoleBindingErr == nil {
		// Delete user Role Binding
		if err := r.Delete(context.TODO(), userRoleBinding); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Role Binding", userRoleBinding.Name)
	}

	//Success
	return reconcile.Result{}, nil
}
