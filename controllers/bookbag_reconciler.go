package controllers

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	"github.com/stakater/workshop-operator/common/bookbag"
	"github.com/stakater/workshop-operator/common/kubernetes"

	"github.com/stakater/workshop-operator/common/util"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	BOOKBAG_NAMESPACE_NAME    = "workshop-guides"
	BOOKBAG_ROLE_BINDING_NAME = "adim"
	BOOKBAG_ROLE_KIND_NAME    = "Role"
	BOOKBAG_PORT              = 10080
)

var bookbagConfigData = map[string]string{
	"gateway.sh":  "",
	"terminal.sh": "",
	"workshop.sh": "",
}

// Reconciling Bookbag
func (r *WorkshopReconciler) reconcileBookbag(workshop *workshopv1.Workshop, users int,
	appsHostnameSuffix string, openshiftConsoleURL string) (reconcile.Result, error) {
	enabled := workshop.Spec.Infrastructure.Guide.Bookbag.Enabled
	id := 1
	for {
		if id <= users && enabled {

			if result, err := r.addUpdateBookbag(workshop, strconv.Itoa(id),
				appsHostnameSuffix, openshiftConsoleURL); util.IsRequeued(result, err) {
				return result, err
			}
		} else {

			bookbagName := fmt.Sprintf("bookbag-%d", id)

			depFound := &appsv1.Deployment{}
			depErr := r.Get(context.TODO(), types.NamespacedName{Name: bookbagName, Namespace: BOOKBAG_NAMESPACE_NAME}, depFound)

			if depErr != nil && errors.IsNotFound(depErr) {
				break
			}
		}
		id++
	}

	//Success
	return reconcile.Result{}, nil
}

func (r *WorkshopReconciler) addUpdateBookbag(workshop *workshopv1.Workshop, userID string,
	appsHostnameSuffix string, openshiftConsoleURL string) (reconcile.Result, error) {

	// Create Namespace
	namespace := kubernetes.NewNamespace(workshop, r.Scheme, BOOKBAG_NAMESPACE_NAME)
	if err := r.Create(context.TODO(), namespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Project", namespace.Name)
	}

	bookbagName := fmt.Sprintf("user%s-bookbag", userID)
	labels := map[string]string{
		"app":                       bookbagName,
		"app.kubernetes.io/part-of": "bookbag",
	}

	// Create ConfigMap
	envConfigMap := kubernetes.NewConfigMap(workshop, r.Scheme, bookbagName+"-env", BOOKBAG_NAMESPACE_NAME, labels, bookbagConfigData)
	if err := r.Create(context.TODO(), envConfigMap); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s ConfigMap", envConfigMap.Name)
	}

	// Create ConfigMap
	varConfigMap := kubernetes.NewConfigMap(workshop, r.Scheme, bookbagName+"-vars", BOOKBAG_NAMESPACE_NAME, labels, nil)
	if err := r.Create(context.TODO(), varConfigMap); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s ConfigMap", varConfigMap.Name)
	}

	// Create Service Account
	serviceAccount := kubernetes.NewServiceAccount(workshop, r.Scheme, bookbagName, BOOKBAG_NAMESPACE_NAME, labels)
	if err := r.Create(context.TODO(), serviceAccount); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Service Account", serviceAccount.Name)
	}

	// Create Role Binding
	roleBinding := kubernetes.NewRoleBindingSA(workshop, r.Scheme, bookbagName, BOOKBAG_NAMESPACE_NAME, labels,
		serviceAccount.Name, BOOKBAG_ROLE_BINDING_NAME, BOOKBAG_ROLE_KIND_NAME)
	if err := r.Create(context.TODO(), roleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Role Binding", roleBinding.Name)
	}

	// Deploy/Update Bookbag
	dep := bookbag.NewDeployment(workshop, r.Scheme, bookbagName, BOOKBAG_NAMESPACE_NAME, labels, userID, appsHostnameSuffix, openshiftConsoleURL)
	if err := r.Create(context.TODO(), dep); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Deployment", dep.Name)
	} else if errors.IsAlreadyExists(err) {
		deploymentFound := &appsv1.Deployment{}
		if err := r.Get(context.TODO(), types.NamespacedName{Name: dep.Name, Namespace: BOOKBAG_NAMESPACE_NAME}, deploymentFound); err != nil {
			return reconcile.Result{}, err
		} else if err == nil {
			if !reflect.DeepEqual(dep.Spec.Template.Spec.Containers[0].Env, deploymentFound.Spec.Template.Spec.Containers[0].Env) {
				// Update Guide
				if err := r.Update(context.TODO(), dep); err != nil {
					return reconcile.Result{}, err
				}
				log.Infof("Updated %s Deployment", dep.Name)
			}
		}
	}

	// Create Service
	service := kubernetes.NewService(workshop, r.Scheme, bookbagName, BOOKBAG_NAMESPACE_NAME, labels, []string{"http"}, []int32{BOOKBAG_PORT})
	if err := r.Create(context.TODO(), service); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Service", service.Name)
	}

	// Create Route
	route := kubernetes.NewRoute(workshop, r.Scheme, bookbagName, BOOKBAG_NAMESPACE_NAME, labels, bookbagName, BOOKBAG_PORT)
	if err := r.Create(context.TODO(), route); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Route", route.Name)
	}

	//Success
	return reconcile.Result{}, nil
}

func (r *WorkshopReconciler) deleteBookbag(workshop *workshopv1.Workshop, userID int, appsHostnameSuffix string, openshiftConsoleURL string) (reconcile.Result, error) {

	id := 1
	for {
		bookbagName := fmt.Sprintf("user%s-bookbag", strconv.Itoa(id))
		labels := map[string]string{
			"app":                       bookbagName,
			"app.kubernetes.io/part-of": "bookbag",
		}

		route := kubernetes.NewRoute(workshop, r.Scheme, bookbagName, BOOKBAG_NAMESPACE_NAME, labels, bookbagName, BOOKBAG_PORT)
		// Delete route
		if err := r.Delete(context.TODO(), route); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Route", route.Name)

		service := kubernetes.NewService(workshop, r.Scheme, bookbagName, BOOKBAG_NAMESPACE_NAME, labels, []string{"http"}, []int32{BOOKBAG_PORT})
		// Delete Service
		if err := r.Delete(context.TODO(), service); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Service", service.Name)

		dep := bookbag.NewDeployment(workshop, r.Scheme, bookbagName, BOOKBAG_NAMESPACE_NAME, labels, strconv.Itoa(userID), appsHostnameSuffix, openshiftConsoleURL)
		// Delete Deployment
		if err := r.Delete(context.TODO(), dep); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Deployment", dep.Name)

		serviceAccount := kubernetes.NewServiceAccount(workshop, r.Scheme, bookbagName, BOOKBAG_NAMESPACE_NAME, labels)

		roleBinding := kubernetes.NewRoleBindingSA(workshop, r.Scheme, bookbagName, BOOKBAG_NAMESPACE_NAME, labels,
			serviceAccount.Name, BOOKBAG_ROLE_BINDING_NAME, BOOKBAG_ROLE_KIND_NAME)
		//Delete  Role Binding
		if err := r.Delete(context.TODO(), roleBinding); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s RoleBinding", roleBinding.Name)

		// Delete  Service Account
		if err := r.Delete(context.TODO(), serviceAccount); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Service Account", serviceAccount.Name)

		varConfigMap := kubernetes.NewConfigMap(workshop, r.Scheme, bookbagName+"-vars", BOOKBAG_NAMESPACE_NAME, labels, nil)
		// Delete ConfigMap
		if err := r.Delete(context.TODO(), varConfigMap); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s ConfigMap", varConfigMap.Name)

		envConfigMap := kubernetes.NewConfigMap(workshop, r.Scheme, bookbagName+"-env", BOOKBAG_NAMESPACE_NAME, labels, bookbagConfigData)
		// Delete ConfigMap
		if err := r.Delete(context.TODO(), envConfigMap); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s ConfigMap", envConfigMap.Name)

		depFound := &appsv1.Deployment{}
		depErr := r.Get(context.TODO(), types.NamespacedName{Name: bookbagName, Namespace: BOOKBAG_NAMESPACE_NAME}, depFound)
		if depErr != nil && errors.IsNotFound(depErr) {
			break
		}
		id++
	}

	namespace := kubernetes.NewNamespace(workshop, r.Scheme, BOOKBAG_NAMESPACE_NAME)
	// delete namespace
	if err := r.Delete(context.TODO(), namespace); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s namespace", namespace.Name)

	return reconcile.Result{}, nil
	//Success

}
