package controllers

import (
	"context"
	"encoding/base64"
	"fmt"
	userv1 "github.com/openshift/api/user/v1"
	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	openshiftuser "github.com/stakater/workshop-operator/common/user"
	"github.com/stakater/workshop-operator/common/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"os/exec"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
)

const (
	HTPASSWD_SECRET_NAME             = "htpass-workshop-users"
	HTPASSWD_SECRET_NAMESPACE_NAME   = "openshift-config"
	USER_ROLE_BINDING_NAMESPACE_NAME = "workshop-infra"
	IDENTITY_NAME                    = "htpass-workshop-users"
	USER_IDENTITY_MAPPING_NAME       = "htpass-workshop-users"
	USER_LABEL_SELECTOR              = "createdBy=WorkshopOperator"
)

var userLabels = map[string]string{
	"createdBy": "WorkshopOperator",
}

func (r *WorkshopReconciler) reconcileUser(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	createUsers := make(map[string]bool)
	totalUsers := workshop.Spec.UserDetails.NumberOfUsers
	userPrefix := workshop.Spec.UserDetails.UserNamePrefix
	for userSuffix := 1; userSuffix <= totalUsers; userSuffix++ {
		userName := fmt.Sprint(userPrefix, userSuffix)
		createUsers[userName] = true
	}

	listUsers, err := r.createdUserList(workshop)
	if err != nil {
		log.Errorf("Failed to get Created User List {%s} ", err)
	}

	for _, user := range listUsers.Items {
		username := user.Name
		_, ok := createUsers[username]
		if ok {
			createUsers[username] = false
		} else {
			if result, err := r.deleteOpenshiftUser(workshop, r.Scheme, username); util.IsRequeued(result, err) {
				return result, err
			}
		}
	}

	for username, value := range createUsers {
		if value {
			if result, err := r.addUser(workshop, r.Scheme, username); util.IsRequeued(result, err) {
				return result, err
			}
		}
	}
	if result, err := r.createUserHtpasswd(workshop, createUsers); util.IsRequeued(result, err) {
		return result, err
	}

	//Success
	return reconcile.Result{}, nil
}

// Add user in openshift cluster
func (r *WorkshopReconciler) addUser(workshop *workshopv1.Workshop, scheme *runtime.Scheme, username string) (reconcile.Result, error) {

	//Create User
	user := openshiftuser.NewUser(workshop, r.Scheme, username, userLabels)
	if err := r.Create(context.TODO(), user); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s User", user.Name)
	}

	// Create User Role Binding
	userRoleBinding := openshiftuser.NewUserRoleBinding(workshop, r.Scheme, username, USER_ROLE_BINDING_NAMESPACE_NAME,
		USER_ROLE_BINDING_NAME, KIND_CLUSTER_ROLE)
	if err := r.Create(context.TODO(), userRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Role Binding", userRoleBinding.Name)
	}

	// Get User
	userFound := &userv1.User{}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: username}, userFound); err != nil {
		log.Errorf("Failed to find %s User", userFound.Name)

	}

	// Create Identity
	identity := openshiftuser.NewIdentity(workshop, r.Scheme, username, IDENTITY_NAME, userFound)
	if err := r.Create(context.TODO(), identity); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Identity ", identity.Name)
	}

	// Create User Identity Mapping
	userIdentity := openshiftuser.NewUserIdentityMapping(workshop, r.Scheme, USER_IDENTITY_MAPPING_NAME, username)
	if err := r.Create(context.TODO(), userIdentity); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s User Identity Mapping ", userIdentity.Name)
	}

	//Success
	return reconcile.Result{}, nil
}

func (r *WorkshopReconciler) createUserHtpasswd(workshop *workshopv1.Workshop, users map[string]bool) (reconcile.Result, error) {

	var htpasswds []byte
	var countUsers int
	createUsers := []string{}

	password := workshop.Spec.UserDetails.DefaultPassword
	totalUsers := workshop.Spec.UserDetails.NumberOfUsers
	userPrefix := workshop.Spec.UserDetails.UserNamePrefix

	for usersname := range users {
		createUsers = append(createUsers, usersname)
	}

	for _, username := range createUsers {
		command := "echo \"password\" | htpasswd -b -B -i -n " + username
		updateCommad := fmt.Sprint(strings.Replace(command, "password", password, -1))
		out, err := exec.Command("/bin/bash", "-c", updateCommad).Output()
		if err != nil {
			log.Errorf("Failed to Execute Bash Command %s", err)
		}
		userpwd := fmt.Sprint(strings.TrimSpace(string(out)), "\n")
		htpasswds = append(htpasswds, []byte(userpwd)...)
	}

	htpasswdSecret := openshiftuser.NewHTPasswdSecret(workshop, r.Scheme, HTPASSWD_SECRET_NAME, HTPASSWD_SECRET_NAMESPACE_NAME, htpasswds)
	if err := r.Create(context.TODO(), htpasswdSecret); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created  %s Secret", htpasswdSecret.Name)
	} else if errors.IsAlreadyExists(err) {
		// Get secret
		secretFound := &corev1.Secret{}
		if err := r.Get(context.TODO(), types.NamespacedName{Name: HTPASSWD_SECRET_NAME, Namespace: HTPASSWD_SECRET_NAMESPACE_NAME}, secretFound); err == nil {
			for _, secretData := range secretFound.Data {
				encodedSecret := base64.StdEncoding.EncodeToString(secretData)
				decodeSecret, err := base64.StdEncoding.DecodeString(encodedSecret)
				if err != nil {
					log.Errorf("Failed to Decode Secret %s", err)
				}
				countUsers = strings.Count(string(decodeSecret), userPrefix)
			}
			if totalUsers > countUsers || totalUsers < countUsers {
				if err := r.Delete(context.TODO(), htpasswdSecret); err != nil {
					return reconcile.Result{}, err
				}
				if err := r.Create(context.TODO(), htpasswdSecret); err != nil && !errors.IsAlreadyExists(err) {
					return reconcile.Result{}, err
				} else if err == nil {
					log.Infof("Created  %s secret", htpasswdSecret.Name)
				}
			}
		}
	}

	return reconcile.Result{}, nil
}

// deleteUsers delete users in openshift cluster
func (r *WorkshopReconciler) deleteUsers(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	listUsers, err := r.createdUserList(workshop)
	if err != nil {
		log.Errorf("Failed to get Created User List {%s} ", err)
	}

	for _, user := range listUsers.Items {
		username := user.Name
		if result, err := r.deleteOpenshiftUser(workshop, r.Scheme, username); util.IsRequeued(result, err) {
			return result, err
		}
	}

	if result, err := r.deleteUserHtpasswd(workshop); err != nil {
		return result, err
	}

	//Success
	return reconcile.Result{}, nil
}

// deleteUser delete OpenShift user
func (r *WorkshopReconciler) deleteOpenshiftUser(workshop *workshopv1.Workshop, scheme *runtime.Scheme, username string) (reconcile.Result, error) {

	// Get user
	userFound := &userv1.User{}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: username}, userFound); err != nil {
		log.Errorf("Failed to find %s User", userFound.Name)
	}
	//
	// Delete User Identity Mapping
	userIdentity := openshiftuser.NewUserIdentityMapping(workshop, r.Scheme, USER_IDENTITY_MAPPING_NAME, username)
	if err := r.Delete(context.TODO(), userIdentity); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s User Identity Mapping ", userIdentity.Name)

	// Delete Identity
	identity := openshiftuser.NewIdentity(workshop, r.Scheme, username, IDENTITY_NAME, userFound)
	if err := r.Delete(context.TODO(), identity); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Identity  ", identity.Name)

	// Delete User Role Binding
	userRoleBinding := openshiftuser.NewUserRoleBinding(workshop, r.Scheme, username, USER_ROLE_BINDING_NAMESPACE_NAME,
		USER_ROLE_BINDING_NAME, KIND_CLUSTER_ROLE)
	if err := r.Delete(context.TODO(), userRoleBinding); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Role Binding", userRoleBinding.Name)

	// Delete User
	user := openshiftuser.NewUser(workshop, r.Scheme, username, userLabels)
	if err := r.Delete(context.TODO(), user); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s user", user.Name)

	//Success
	return reconcile.Result{}, nil
}

//deleteUserHtpasswd delete Htpasswd secret for users
func (r *WorkshopReconciler) deleteUserHtpasswd(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	htpasswdSecret := openshiftuser.NewHTPasswdSecret(workshop, r.Scheme, HTPASSWD_SECRET_NAME, HTPASSWD_SECRET_NAMESPACE_NAME, []byte(""))
	if err := r.Delete(context.TODO(), htpasswdSecret); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s HTPasswd Secret", htpasswdSecret.Name)
	//Success
	return reconcile.Result{}, nil
}

//createdUserList return list of users
func (r *WorkshopReconciler) createdUserList(workshop *workshopv1.Workshop) (*userv1.UserList, error) {
	labelSelector, err := labels.Parse(USER_LABEL_SELECTOR)
	if err != nil {
		log.Errorf("Failed to get list of users %s", err)
	}
	listUsers := &userv1.UserList{}
	listOps := &client.ListOptions{
		LabelSelector: labelSelector,
	}
	// list User
	if err := r.List(context.TODO(), listUsers, listOps); err != nil {
		log.Errorf("Failed to get  list of  users, filtered by labelSelector {%s} ,{%s}", labelSelector, err)
	}
	return listUsers, err
}
