package controllers

import (
	"context"
	"fmt"
	userv1 "github.com/openshift/api/user/v1"
	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	openshiftuser "github.com/stakater/workshop-operator/common/user"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	HTPASSWD_SECRET_NAME            = "htpass-workshop-users"
	HTPASSWD_SECRET_NAMESPACE_NAME  = "openshift-config"
	USER_ROLE_BINDIN_NAMESPACE_NAME = "workshop-infra"
	IDENTITY_NAME                   = "htpass-workshop-users"
	USER_IDENTITY_MAPPING_NAME      = "htpass-workshop-users"
)

var userLabels = map[string]string{
	"createdBy": "WorkshopOperator",
}
var UserLabelSelector = "createdBy=WorkshopOperator"

func (r *WorkshopReconciler) reconcileUser(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	totalUsers := workshop.Spec.UserDetails.NumberOfUsers
	userPrefix := workshop.Spec.UserDetails.UserNamePrefix
	//password := workshop.Spec.UserDetails.DefaultPassword
	var createUsers []string
	var userList []string
	var skipUsers []string

	for userSuffix := 1; userSuffix <= totalUsers; userSuffix++ {
		userName := fmt.Sprint(userPrefix, userSuffix)
		createUsers = append(createUsers, userName)
	}
	labelSelector, err := labels.Parse(UserLabelSelector)
	if err != nil {
		log.Errorf("Error %s", err)
	}

	// list User
	ListUsers := &userv1.UserList{}
	listOps := &client.ListOptions{
		LabelSelector: labelSelector,
	}
	if err := r.List(context.TODO(), ListUsers, listOps); err != nil {
		log.Errorf("Error %s", err)
	}

	for _, user := range ListUsers.Items {
		username := user.Name
		userList = append(userList, username)
	}

	if len(userList) > 0 {
		for _, availableUser := range userList {
			for _, username := range createUsers {
				if availableUser == username {
					skipUsers = append(skipUsers, availableUser)
				}
			}
		}
	}

	for _, username := range createUsers {
		if len(skipUsers) >= 0 {
			//Create User
			user := openshiftuser.NewUser(workshop, r.Scheme, username, userLabels)
			if err := r.Create(context.TODO(), user); err != nil && !errors.IsAlreadyExists(err) {
				return reconcile.Result{}, err
			} else if err == nil {
				log.Infof("Created %s user", user.Name)
			}
		} else {
			for _, availableUser := range skipUsers {
				if availableUser != username {
					user := openshiftuser.NewUser(workshop, r.Scheme, username, userLabels)
					if err := r.Create(context.TODO(), user); err != nil && !errors.IsAlreadyExists(err) {
						return reconcile.Result{}, err
					} else if err == nil {
						log.Infof("Created %s user", user.Name)
					}
				}
			}
		}
	}

	if len(userList) > 0 {
		for _, availableUser := range userList {
			for _, username := range createUsers {
				if availableUser == username {
					skipUsers = append(skipUsers, availableUser)
				}
			}
		}
	}

	createdUsers := len(userList)
	for totalUsers < createdUsers {
		username := fmt.Sprint(userPrefix, createdUsers)
		log.Infoln("delete user", username)
		createdUsers--
	}

	//if result, err := r.CreateUserHtpasswd(workshop, len(userList), createUsers, password); util.IsRequeued(result, err) {
	//	return result, err
	//}

	//Success
	return reconcile.Result{}, nil
}

//func (r *WorkshopReconciler) CreateUserHtpasswd(workshop *workshopv1.Workshop, userList int, createUsers []string, password string) (reconcile.Result, error) {
//	var htpasswds []byte
//
//	if userList == 0 || len(createUsers) > userList || len(createUsers) < userList {
//		for _, username := range createUsers {
//			command := "echo \"password\" | htpasswd -b -B -i -n " + username
//			updateCommad := fmt.Sprint(strings.Replace(command, "password", password, -1))
//			out, err := exec.Command("/bin/bash", "-c", updateCommad).Output()
//			if err != nil {
//				log.Errorf("error %s", err)
//			}
//			userpwd := fmt.Sprint(strings.TrimSpace(string(out)), "\n")
//			htpasswds = append(htpasswds, []byte(userpwd)...)
//		}
//	}
//
//	log.Infoln(string(htpasswds))
//
//	// Get secret
//	secretFound := &corev1.Secret{}
//	if err := r.Get(context.TODO(), types.NamespacedName{Name: HTPASSWD_SECRET_NAME, Namespace: HTPASSWD_SECRET_NAMESPACE_NAME}, secretFound); err != nil {
//		htpasswdSecret := openshiftuser.NewHTPasswdSecret(workshop, r.Scheme, HTPASSWD_SECRET_NAME, HTPASSWD_SECRET_NAMESPACE_NAME, htpasswds)
//		if err := r.Create(context.TODO(), htpasswdSecret); err != nil && !errors.IsAlreadyExists(err) {
//			return reconcile.Result{}, err
//		}
//	} else {
//		patch := client.MergeFrom(secretFound.DeepCopy())
//		secretFound.Data = map[string][]byte{
//			"htpasswd": htpasswds,
//		}
//
//		if err := r.Patch(context.TODO(), secretFound, patch); err != nil {
//			return reconcile.Result{}, err
//		}
//		log.Infof("patched %s Secret", secretFound.Name)
//	}
//
//	return reconcile.Result{}, nil
//}

// Add user in openshift cluster
//func (r *WorkshopReconciler) addUser(workshop *workshopv1.Workshop, scheme *runtime.Scheme, username string) (reconcile.Result, error) {
//	//password := workshop.Spec.UserDetails.DefaultPassword
//	//var htpasswds []byte
//
//	// Create User
//	//user := openshiftuser.NewUser(workshop, r.Scheme, username, userLabels)
//	//if err := r.Create(context.TODO(), user); err != nil && !errors.IsAlreadyExists(err) {
//	//	return reconcile.Result{}, err
//	//} else if err == nil {
//	//	log.Infof("Created %s user", user.Name)
//	//}
//
//	// Create User
//	//
//	//command := "echo \"password\" | htpasswd -b -B -i -n " + username
//	//updateCommad := fmt.Sprint(strings.Replace(command, "password", password, -1))
//	//out, err := exec.Command("/bin/bash", "-c", updateCommad).Output()
//	//if err != nil {
//	//	log.Errorf("error %s", err)
//	//}
//	//userpwd := fmt.Sprint(strings.TrimSpace(string(out)), "\n")
//	//htpasswds = append(htpasswds, []byte(userpwd)...)
//	//
//	//htpasswdSecret := openshiftuser.NewHTPasswdSecret(workshop, r.Scheme, HTPASSWD_SECRET_NAME, HTPASSWD_SECRET_NAMESPACE_NAME, htpasswds)
//	//if err := r.Create(context.TODO(), htpasswdSecret); err != nil && !errors.IsAlreadyExists(err) {
//	//	return reconcile.Result{}, err
//	//} else if errors.IsAlreadyExists(err) {
//	//
//	//	log.Info("Patch HTPasswdSecret")
//	//	// Patch IdentityProvider
//	//	SecretFound := &corev1.Secret{}
//	//	if err := r.Get(context.TODO(), types.NamespacedName{Name: HTPASSWD_SECRET_NAME, Namespace: HTPASSWD_SECRET_NAMESPACE_NAME}, SecretFound); err != nil {
//	//		return reconcile.Result{}, err
//	//	}
//	//	patch := client.MergeFrom(SecretFound.DeepCopy())
//	//	SecretFound.Data = map[string][]byte{
//	//		"htpasswd": htpasswds,
//	//	}
//	//
//	//	if err := r.Patch(context.TODO(), SecretFound, patch); err != nil {
//	//		return reconcile.Result{}, err
//	//	}
//	//	log.Infof("patched %s Secret", SecretFound.Name)
//	//
//	//}
//	//// Create User Role Binding
//	//userRoleBinding := openshiftuser.NewRoleBindingUser(workshop, r.Scheme, username, USER_ROLE_BINDIN_NAMESPACE_NAME,
//	//	USER_ROLE_BINDING_NAME, KIND_CLUSTER_ROLE)
//	//if err := r.Create(context.TODO(), userRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
//	//	return reconcile.Result{}, err
//	//} else if err == nil {
//	//	log.Infof("Created %s Role Binding", userRoleBinding.Name)
//	//}
//	//
//	//// Get User
//	//userFound := &userv1.User{}
//	//if err := r.Get(context.TODO(), types.NamespacedName{Name: username}, userFound); err != nil {
//	//	log.Errorf("Failed to find %s User", userFound.Name)
//	//
//	//}
//	//
//	//// Create Identity
//	//identity := openshiftuser.NewIdentity(workshop, r.Scheme, username, IDENTITY_NAME, userFound)
//	//if err := r.Create(context.TODO(), identity); err != nil && !errors.IsAlreadyExists(err) {
//	//	return reconcile.Result{}, err
//	//} else if err == nil {
//	//	log.Infof("Created %s Identity ", identity.Name)
//	//}
//	//
//	//// Create User Identity Mapping
//	//userIdentity := openshiftuser.NewUserIdentityMapping(workshop, r.Scheme, USER_IDENTITY_MAPPING_NAME, username)
//	//if err := r.Create(context.TODO(), userIdentity); err != nil && !errors.IsAlreadyExists(err) {
//	//	return reconcile.Result{}, err
//	//} else if err == nil {
//	//	log.Infof("Created %s User Identity Mapping ", userIdentity.Name)
//	//}
//
//	//Success
//	return reconcile.Result{}, nil
//}

// CreateUserHtpasswd create Htpasswd secret for users
//func (r *WorkshopReconciler) CreateUserHtpasswd(workshop *workshopv1.Workshop) (reconcile.Result, error) {
//
//	users := workshop.Spec.UserDetails.NumberOfUsers
//	userPrefix := workshop.Spec.UserDetails.UserNamePrefix
//	password := workshop.Spec.UserDetails.DefaultPassword
//	var htpasswds []byte
//
//	for id := 1; id <= users; id++ {
//		username := fmt.Sprint(userPrefix, id)
//		command := "echo \"password\" | htpasswd -b -B -i -n " + username
//		updateCommad := fmt.Sprint(strings.Replace(command, "password", password, -1))
//		out, err := exec.Command("/bin/bash", "-c", updateCommad).Output()
//		if err != nil {
//			log.Errorf("error %s", err)
//		}
//		userpwd := fmt.Sprint(strings.TrimSpace(string(out)), "\n")
//		htpasswds = append(htpasswds, []byte(userpwd)...)
//	}
//
//	htpasswdSecret := openshiftuser.NewHTPasswdSecret(workshop, r.Scheme, HTPASSWD_SECRET_NAME, HTPASSWD_SECRET_NAMESPACE_NAME, htpasswds)
//	if err := r.Create(context.TODO(), htpasswdSecret); err != nil && !errors.IsNotFound(err) {
//		return reconcile.Result{}, err
//	} else if errors.IsAlreadyExists(err) {
//		htpasswdSecretFound := openshiftuser.NewHTPasswdSecret(workshop, r.Scheme, HTPASSWD_SECRET_NAME, HTPASSWD_SECRET_NAMESPACE_NAME, htpasswds)
//		if err := r.Delete(context.TODO(), htpasswdSecretFound); err != nil {
//			return reconcile.Result{}, err
//		}
//		log.Infof("Deleted %s HTPasswd Secret", htpasswdSecret.Name)
//		if err := r.Create(context.TODO(), htpasswdSecret); err != nil && !errors.IsAlreadyExists(err) {
//			return reconcile.Result{}, err
//		} else {
//			log.Infof("Created %s HTPasswd Secret", htpasswdSecret.Name)
//		}
//	}
//	//Success
//	return reconcile.Result{}, nil
//}

//
// deleteUsers delete users in openshift cluster
//func (r *WorkshopReconciler) deleteUsers(workshop *workshopv1.Workshop) (reconcile.Result, error) {
//
//	users := workshop.Spec.UserDetails.NumberOfUsers
//	userPrefix := workshop.Spec.UserDetails.UserNamePrefix
//
//	id := 1
//	for {
//		username := fmt.Sprint(userPrefix, id)
//		if id <= users {
//			if result, err := r.deleteOpenshiftUser(workshop, r.Scheme, username); util.IsRequeued(result, err) {
//				return result, err
//			}
//		} else {
//			user := openshiftuser.NewUser(workshop, r.Scheme, username)
//			userFound := &userv1.User{}
//			userFoundErr := r.Get(context.TODO(), types.NamespacedName{Name: user.Name}, userFound)
//			if userFoundErr != nil && errors.IsNotFound(userFoundErr) {
//				log.Infof("Failed to find %s User ", user.Name)
//				break
//			}
//		}
//		id++
//	}
//	if result, err := r.DeleteUserHtpasswd(workshop); err != nil {
//		return result, err
//	}
//
//	//Success
//	return reconcile.Result{}, nil
//}
//
//// deleteOpenshiftUser delete OpenShift user
//func (r *WorkshopReconciler) deleteOpenshiftUser(workshop *workshopv1.Workshop, scheme *runtime.Scheme, username string) (reconcile.Result, error) {
//
//	// Get user
//	userFound := &userv1.User{}
//	if err := r.Get(context.TODO(), types.NamespacedName{Name: username}, userFound); err != nil {
//		log.Errorf("Failed to find %s User", userFound.Name)
//	}
//	//
//	// Delete User Identity Mapping
//	userIdentity := openshiftuser.NewUserIdentityMapping(workshop, r.Scheme, USER_IDENTITY_MAPPING_NAME, username)
//	if err := r.Delete(context.TODO(), userIdentity); err != nil {
//		return reconcile.Result{}, err
//	}
//	log.Infof("Deleted %s User Identity Mapping ", userIdentity.Name)
//
//	// Delete Identity
//	identity := openshiftuser.NewIdentity(workshop, r.Scheme, username, IDENTITY_NAME, userFound)
//	if err := r.Delete(context.TODO(), identity); err != nil {
//		return reconcile.Result{}, err
//	}
//	log.Infof("Deleted %s Identity  ", identity.Name)
//
//	// Delete User Role Binding
//	userRoleBinding := openshiftuser.NewRoleBindingUser(workshop, r.Scheme, username, USER_ROLE_BINDIN_NAMESPACE_NAME,
//		USER_ROLE_BINDING_NAME, KIND_CLUSTER_ROLE)
//	if err := r.Delete(context.TODO(), userRoleBinding); err != nil {
//		return reconcile.Result{}, err
//	}
//	log.Infof("Deleted %s Role Binding", userRoleBinding.Name)
//
//	// Delete User
//	user := openshiftuser.NewUser(workshop, r.Scheme, username)
//	if err := r.Delete(context.TODO(), user); err != nil {
//		return reconcile.Result{}, err
//	}
//	log.Infof("Deleted %s user", user.Name)
//
//	//Success
//	return reconcile.Result{}, nil
//}

//DeleteUserHTPasswd delete Htpasswd secret for users
func (r *WorkshopReconciler) DeleteUserHtpasswd(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	htpasswdSecret := openshiftuser.NewHTPasswdSecret(workshop, r.Scheme, HTPASSWD_SECRET_NAME, HTPASSWD_SECRET_NAMESPACE_NAME, []byte(""))
	if err := r.Delete(context.TODO(), htpasswdSecret); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s HTPasswd Secret", htpasswdSecret.Name)
	//Success
	return reconcile.Result{}, nil
}
