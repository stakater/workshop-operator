package controllers

import (
	"context"
	"fmt"
	userv1 "github.com/openshift/api/user/v1"
	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	openshiftuser "github.com/stakater/workshop-operator/common/user"
	"github.com/stakater/workshop-operator/common/util"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *WorkshopReconciler) reconcileUser(workshop *workshopv1.Workshop, users int) (reconcile.Result, error) {

	id := 1
	for {
		username := fmt.Sprintf("user%d", id)
		if id <= users {
			if result, err := r.addUser(workshop, r.Scheme, username); util.IsRequeued(result, err) {
				return result, err
			}
		} else {
			user := openshiftuser.NewUser(workshop, r.Scheme, username)
			userFound := &userv1.User{}
			userFoundErr := r.Get(context.TODO(), types.NamespacedName{Name: user.Name}, userFound)

			if userFoundErr != nil && errors.IsNotFound(userFoundErr) {
				break
			}
		}
		id++
	}
	//Success
	return reconcile.Result{}, nil
}

func (r *WorkshopReconciler) addUser(workshop *workshopv1.Workshop, scheme *runtime.Scheme, username string) (reconcile.Result, error) {

	user := openshiftuser.NewUser(workshop, r.Scheme, username)
	if err := r.Create(context.TODO(), user); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s user", user.Name)
	}

	// Create User Role Binding
	userRoleBinding := openshiftuser.NewRoleBindingUsers(workshop, r.Scheme, username, "workshop-infra",
		USER_ROLE_BINDING_NAME, KIND_CLUSTER_ROLE)
	if err := r.Create(context.TODO(), userRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Role Binding", userRoleBinding.Name)
	}
	//Success
	return reconcile.Result{}, nil
}
