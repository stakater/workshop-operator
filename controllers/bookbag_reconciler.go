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
	BOOKBAGNAMESPACENAME       = "workshop-guides"
	BOOKBAGROLEBINDINGNAME     = "adim"
	BOOKBAGCLUSTERROLEKINDNAME = "Role"
	BOOKBAGRouteNUMBER         = 10080
)

// Reconciling Bookbag
func (r *WorkshopReconciler) reconcileBookbag(workshop *workshopv1.Workshop, users int,
	appsHostnameSuffix string, openshiftConsoleURL string) (reconcile.Result, error) {
	enabled := workshop.Spec.Infrastructure.Guide.Bookbag.Enabled

	id := 1
	for {
		if id <= users && enabled {
			// Bookback
			if result, err := r.addUpdateBookbag(workshop, strconv.Itoa(id),
				appsHostnameSuffix, openshiftConsoleURL); util.IsRequeued(result, err) {
				return result, err
			}
		} else {

			bookbagName := fmt.Sprintf("bookbag-%d", id)

			depFound := &appsv1.Deployment{}
			depErr := r.Get(context.TODO(), types.NamespacedName{Name: bookbagName, Namespace: BOOKBAGNAMESPACENAME}, depFound)

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

	namespace := kubernetes.NewNamespace(workshop, r.Scheme, BOOKBAGNAMESPACENAME)
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
	data := map[string]string{
		"gateway.sh":  "",
		"terminal.sh": "",
		"workshop.sh": "",
	}

	envConfigMap := kubernetes.NewConfigMap(workshop, r.Scheme, bookbagName+"-env", BOOKBAGNAMESPACENAME, labels, data)
	if err := r.Create(context.TODO(), envConfigMap); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s bookbag ConfigMap", envConfigMap.Name)
	}

	varConfigMap := kubernetes.NewConfigMap(workshop, r.Scheme, bookbagName+"-vars", BOOKBAGNAMESPACENAME, labels, nil)
	if err := r.Create(context.TODO(), varConfigMap); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s bookbag ConfigMap", varConfigMap.Name)
	}

	// Create Service Account
	serviceAccount := kubernetes.NewServiceAccount(workshop, r.Scheme, bookbagName, BOOKBAGNAMESPACENAME, labels)
	if err := r.Create(context.TODO(), serviceAccount); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s bookbag Service Account", serviceAccount.Name)
	}

	// Create Role Binding
	roleBinding := kubernetes.NewRoleBindingSA(workshop, r.Scheme, bookbagName, BOOKBAGNAMESPACENAME, labels,
		serviceAccount.Name, BOOKBAGROLEBINDINGNAME, BOOKBAGCLUSTERROLEKINDNAME)
	if err := r.Create(context.TODO(), roleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s bookbag Role Binding", roleBinding.Name)
	}

	// Deploy/Update Bookbag
	dep := bookbag.NewDeployment(workshop, r.Scheme, bookbagName, BOOKBAGNAMESPACENAME, labels, userID, appsHostnameSuffix, openshiftConsoleURL)
	if err := r.Create(context.TODO(), dep); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Deployment", dep.Name)
	} else if errors.IsAlreadyExists(err) {
		deploymentFound := &appsv1.Deployment{}
		if err := r.Get(context.TODO(), types.NamespacedName{Name: dep.Name, Namespace: BOOKBAGNAMESPACENAME}, deploymentFound); err != nil {
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
	service := kubernetes.NewService(workshop, r.Scheme, bookbagName, BOOKBAGNAMESPACENAME, labels, []string{"http"}, []int32{BOOKBAGRouteNUMBER})
	if err := r.Create(context.TODO(), service); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s bookbag Service", service.Name)
	}

	// Create Route
	route := kubernetes.NewRoute(workshop, r.Scheme, bookbagName, BOOKBAGNAMESPACENAME, labels, bookbagName, BOOKBAGRouteNUMBER)
	if err := r.Create(context.TODO(), route); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s bookbag  Route", route.Name)
	}

	//Success
	return reconcile.Result{}, nil
}

func (r *WorkshopReconciler) deleteBookbag(workshop *workshopv1.Workshop, userID int, appsHostnameSuffix string, openshiftConsoleURL string) (reconcile.Result, error) {
	//enabled := workshop.Spec.Infrastructure.Guide.Bookbag.Enabled

	id := 1
	for {
		bookbagName := fmt.Sprintf("user%s-bookbag", strconv.Itoa(id))
		labels := map[string]string{
			"app":                       bookbagName,
			"app.kubernetes.io/part-of": "bookbag",
		}
		// Create ConfigMap
		data := map[string]string{
			"gateway.sh":  "",
			"terminal.sh": "",
			"workshop.sh": "",
		}

		route := kubernetes.NewRoute(workshop, r.Scheme, bookbagName, BOOKBAGNAMESPACENAME, labels, bookbagName, BOOKBAGRouteNUMBER)
		// Delete route
		if err := r.Delete(context.TODO(), route); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s bookbag route", route.Name)

		service := kubernetes.NewService(workshop, r.Scheme, bookbagName, BOOKBAGNAMESPACENAME, labels, []string{"http"}, []int32{BOOKBAGRouteNUMBER})
		// Delete Service
		if err := r.Delete(context.TODO(), service); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s bookbag Service", service.Name)

		dep := bookbag.NewDeployment(workshop, r.Scheme, bookbagName, BOOKBAGNAMESPACENAME, labels, strconv.Itoa(userID), appsHostnameSuffix, openshiftConsoleURL)
		// Delete Deployment
		if err := r.Delete(context.TODO(), dep); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s bookbag Deployment", dep.Name)

		serviceAccount := kubernetes.NewServiceAccount(workshop, r.Scheme, bookbagName, BOOKBAGNAMESPACENAME, labels)

		roleBinding := kubernetes.NewRoleBindingSA(workshop, r.Scheme, bookbagName, BOOKBAGNAMESPACENAME, labels,
			serviceAccount.Name, BOOKBAGROLEBINDINGNAME, BOOKBAGCLUSTERROLEKINDNAME)
		//Delete  Role Binding
		if err := r.Delete(context.TODO(), roleBinding); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s bookbag roleBinding", roleBinding.Name)

		// Delete  Service Account
		if err := r.Delete(context.TODO(), serviceAccount); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s bookbag Service Account", serviceAccount.Name)

		varConfigMap := kubernetes.NewConfigMap(workshop, r.Scheme, bookbagName+"-vars", BOOKBAGNAMESPACENAME, labels, nil)
		// Delete  var ConfigMap
		if err := r.Delete(context.TODO(), varConfigMap); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s bookbag var ConfigMap", varConfigMap.Name)

		envConfigMap := kubernetes.NewConfigMap(workshop, r.Scheme, bookbagName+"-env", BOOKBAGNAMESPACENAME, labels, data)
		// Delete  env ConfigMap
		if err := r.Delete(context.TODO(), envConfigMap); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s bookbag env ConfigMap", envConfigMap.Name)

		if true {
			bookbagName := fmt.Sprintf("bookbag-%d", id)

			depFound := &appsv1.Deployment{}
			depErr := r.Get(context.TODO(), types.NamespacedName{Name: bookbagName, Namespace: BOOKBAGNAMESPACENAME}, depFound)
			if depErr == nil {
				break
			}
		}
		id++

	}
	return reconcile.Result{}, nil
	//Success

}
