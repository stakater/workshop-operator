package controllers

import (
	"context"
	"fmt"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	openshiftuser "github.com/stakater/workshop-operator/common/user"
	"github.com/stakater/workshop-operator/common/util"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
			break
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

	// Create htpasswd secret
	htpasswdsecret := openshiftuser.NewHTPasswdSecret(workshop, r.Scheme, username)
	if err := r.Create(context.TODO(), htpasswdsecret); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s HTPasswd Secret", htpasswdsecret.Name)
	}

	// Patch Username and  password
	oauthFound := &configv1.OAuth{}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, oauthFound); err != nil {
		log.Error("Failed to get Oauth")
	}
	patch := client.MergeFrom(oauthFound.DeepCopy())
	oauthFound.Spec = configv1.OAuthSpec{
		IdentityProviders: []configv1.IdentityProvider{
			{
				Name:          "htpass-secret" + username,
				MappingMethod: "claim",
				IdentityProviderConfig: configv1.IdentityProviderConfig{
					Type: "HTPasswd",
					HTPasswd: &configv1.HTPasswdIdentityProvider{
						FileData: configv1.SecretNameReference{
							Name: "htpass-secret" + username,
						},
					},
				},
			},
		},
	}

	err := r.Patch(context.TODO(), oauthFound, patch)
	if err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else {
		log.Infof("Patched %s HTPAsswd ", oauthFound.Name)
	}

	//Success
	return reconcile.Result{}, nil
}
