package controllers

import (
	"context"
	"fmt"
	configv1 "github.com/openshift/api/config/v1"
	userv1 "github.com/openshift/api/user/v1"
	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	openshiftuser "github.com/stakater/workshop-operator/common/user"
	"github.com/stakater/workshop-operator/common/util"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *WorkshopReconciler) reconcileUser(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	users := workshop.Spec.UserDetails.NumberOfUsers
	userPrefix := workshop.Spec.UserDetails.UserNamePrefix

	if result, err := r.CreateUserHTPasswd(workshop); err != nil {
		return result, err
	}

	id := 1
	for {
		username := fmt.Sprint(userPrefix, id)
		if id <= users {
			if result, err := r.addUser(workshop, r.Scheme, username, id); util.IsRequeued(result, err) {
				return result, err
			}
		} else {
			user := openshiftuser.NewUser(workshop, r.Scheme, username)
			userFound := &userv1.User{}
			userFoundErr := r.Get(context.TODO(), types.NamespacedName{Name: user.Name}, userFound)
			if userFoundErr != nil && errors.IsNotFound(userFoundErr) {
				log.Errorf("failed to get user %s", user.Name)
				break
			}
		}
		id++
	}
	if result, err := r.PatchOauth(workshop); err != nil {
		return result, err
	}
	//Success
	return reconcile.Result{}, nil
}

// CreateUserHTPasswd create HTPasswd
func (r *WorkshopReconciler) CreateUserHTPasswd(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	users := workshop.Spec.UserDetails.NumberOfUsers
	userPrefix := workshop.Spec.UserDetails.UserNamePrefix

	for id := 1; id <= users; id++ {
		username := fmt.Sprint(userPrefix, id)
		openshiftuser.GeneratePasswd(workshop, username)
	}
	htpasswdFile, err := ioutil.ReadFile("hack/htpasswdfile.txt")
	if err != nil {
		log.Errorf(err.Error())
	}

	// Create htpasswd secret
	htpasswdsecret := openshiftuser.NewHTPasswdSecret(workshop, r.Scheme, htpasswdFile)
	if err := r.Create(context.TODO(), htpasswdsecret); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s HTPasswd Secret", htpasswdsecret.Name)
	}

	deleteHtpasswdFile := os.Remove("hack/htpasswdfile.txt")
	if deleteHtpasswdFile != nil {
		log.Fatal(deleteHtpasswdFile)
	}

	//Success
	return reconcile.Result{}, nil
}

// Add user to openshift
func (r *WorkshopReconciler) addUser(workshop *workshopv1.Workshop, scheme *runtime.Scheme, username string, id int) (reconcile.Result, error) {

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

	// Get User
	userFound := &userv1.User{}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: username}, userFound); err != nil {
		log.Error("Failed to get User")
	}

	// Create Identity
	identity := openshiftuser.NewIdentity(workshop, r.Scheme, username, userFound)
	if err := r.Create(context.TODO(), identity); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Identity ", identity.Name)
	}

	// Create User Identity Mapping
	userIdentity := openshiftuser.NewUserIdentityMapping(workshop, r.Scheme, username)
	if err := r.Create(context.TODO(), userIdentity); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s User Identity Mapping ", userIdentity.Name)
	}

	//Success
	return reconcile.Result{}, nil
}

func (r *WorkshopReconciler) deleteUsers(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	users := workshop.Spec.UserDetails.NumberOfUsers
	userPrefix := workshop.Spec.UserDetails.UserNamePrefix
	if result, err := r.DeleteUserHTPasswd(workshop); err != nil {
		return result, err
	}
	id := 1
	for {
		username := fmt.Sprint(userPrefix, id)
		if id <= users {
			if result, err := r.deleteOpenshiftUser(workshop, r.Scheme, username, id); util.IsRequeued(result, err) {
				return result, err
			}
		} else {
			log.Info("deleteUsers else")
			break
		}
		id++
	}

	//Success
	return reconcile.Result{}, nil
}

// deleteOpenshiftUser delete OpenShift user
func (r *WorkshopReconciler) deleteOpenshiftUser(workshop *workshopv1.Workshop, scheme *runtime.Scheme, username string, id int) (reconcile.Result, error) {

	// Get user
	userFound := &userv1.User{}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: username}, userFound); err != nil {
		log.Error("Failed to get User")
	}

	// Delete User Identity Mapping
	userIdentity := openshiftuser.NewUserIdentityMapping(workshop, r.Scheme, username)
	if err := r.Delete(context.TODO(), userIdentity); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s User Identity Mapping ", userIdentity.Name)

	// Delete Identity
	identity := openshiftuser.NewIdentity(workshop, r.Scheme, username, userFound)
	if err := r.Delete(context.TODO(), identity); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Identity  ", identity.Name)

	// Delete User Role Binding
	userRoleBinding := openshiftuser.NewRoleBindingUsers(workshop, r.Scheme, username, "workshop-infra",
		USER_ROLE_BINDING_NAME, KIND_CLUSTER_ROLE)
	if err := r.Delete(context.TODO(), userRoleBinding); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Role Binding", userRoleBinding.Name)

	user := openshiftuser.NewUser(workshop, r.Scheme, username)
	if err := r.Delete(context.TODO(), user); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s user", user.Name)

	//Success
	return reconcile.Result{}, nil
}

// DeleteUserHTPasswd delete User HTPasswd
func (r *WorkshopReconciler) DeleteUserHTPasswd(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	htpasswdSecret := openshiftuser.NewHTPasswdSecret(workshop, r.Scheme, []byte(""))
	if err := r.Delete(context.TODO(), htpasswdSecret); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s HTPasswd Secret", htpasswdSecret.Name)
	//Success
	return reconcile.Result{}, nil
}
func (r *WorkshopReconciler) PatchOauth(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	// Patch IdentityProvider
	oauthFound := &configv1.OAuth{}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, oauthFound); err != nil {
		log.Error("Failed to get Oauth")
	}
	patch := client.MergeFrom(oauthFound.DeepCopy())
	IdentityProvider := []configv1.IdentityProvider{
		{
			Name:          "htpass-workshop-users",
			MappingMethod: "claim",
			IdentityProviderConfig: configv1.IdentityProviderConfig{
				Type: "HTPasswd",
				HTPasswd: &configv1.HTPasswdIdentityProvider{
					FileData: configv1.SecretNameReference{
						Name: "htpass-workshop-users",
					},
				},
			},
		},
	}
	oauthFound.Spec.IdentityProviders = append(IdentityProvider, oauthFound.Spec.IdentityProviders...)

	err := r.Patch(context.TODO(), oauthFound, patch)
	if err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else {
		log.Infof("Patched %s Identity Providers  ", oauthFound.Name)
	}
	//Success
	return reconcile.Result{}, nil
}
