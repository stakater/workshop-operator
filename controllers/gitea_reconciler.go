package controllers

import (
	"context"
	"crypto/tls"
	"fmt"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	"github.com/stakater/workshop-operator/common/gitea"
	"github.com/stakater/workshop-operator/common/kubernetes"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/stakater/workshop-operator/common/util"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconciling Gitea
func (r *WorkshopReconciler) reconcileGitea(workshop *workshopv1.Workshop, users int) (reconcile.Result, error) {
	enabledGitea := workshop.Spec.Infrastructure.Gitea.Enabled

	giteaNamespaceName := "nexus"

	if enabledGitea {
		if result, err := r.addGitea(workshop, users, giteaNamespaceName); util.IsRequeued(result, err) {
			return result, err
		}
	}

	if enabledGitea {
		if result, err := r.deleteGitea(workshop, users, giteaNamespaceName); util.IsRequeued(result, err) {
			return result, err
		}
	}

	//Success
	return reconcile.Result{}, nil
}

// Add Gitea
func (r *WorkshopReconciler) addGitea(workshop *workshopv1.Workshop, users int, giteaNamespaceName string) (reconcile.Result, error) {

	imageName := workshop.Spec.Infrastructure.Gitea.Image.Name
	imageTag := workshop.Spec.Infrastructure.Gitea.Image.Tag

	labels := map[string]string{
		"app":                       "gitea",
		"app.kubernetes.io/name":    "gitea",
		"app.kubernetes.io/part-of": "gitea",
	}

	// Create Project
	giteaNamespace := kubernetes.NewNamespace(workshop, r.Scheme, giteaNamespaceName)
	if err := r.Create(context.TODO(), giteaNamespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Project", giteaNamespace.Name)
	}

	// Create CRD
	giteaCustomResourceDefinition := kubernetes.NewCustomResourceDefinition(workshop, r.Scheme, "giteas.gpte.opentlc.com", "gpte.opentlc.com", "Gitea", "GiteaList", "giteas", "gitea", "v1alpha1", nil, nil)
	if err := r.Create(context.TODO(), giteaCustomResourceDefinition); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Custom Resource Definition", giteaCustomResourceDefinition.Name)
	}

	// Create Service Account
	giteaServiceAccount := kubernetes.NewServiceAccount(workshop, r.Scheme, "gitea-operator", giteaNamespace.Name, labels)
	if err := r.Create(context.TODO(), giteaServiceAccount); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Service Account", giteaServiceAccount.Name)
	}

	// Create Cluster Role
	giteaClusterRole := kubernetes.NewClusterRole(workshop, r.Scheme, "gitea-operator", giteaNamespace.Name, labels, kubernetes.GiteaRules())
	if err := r.Create(context.TODO(), giteaClusterRole); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Cluster Role", giteaClusterRole.Name)
	}

	// Create Cluster Role Binding
	giteaClusterRoleBinding := kubernetes.NewClusterRoleBindingSA(workshop, r.Scheme, "gitea-operator", giteaNamespace.Name, labels, "gitea-operator", "gitea-operator", "ClusterRole")
	if err := r.Create(context.TODO(), giteaClusterRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Cluster Role Binding", giteaClusterRoleBinding.Name)
	}

	giteaOperator := kubernetes.NewAnsibleOperatorDeployment(workshop, r.Scheme, "gitea-operator", giteaNamespace.Name, labels, imageName+":"+imageTag, "gitea-operator")

	// Create Operator
	if err := r.Create(context.TODO(), giteaOperator); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Operator", giteaOperator.Name)
	}

	// Create Custom Resource
	giteaCustomResource := gitea.NewCustomResource(workshop, r.Scheme, "gitea-server", giteaNamespace.Name, labels)
	if err := r.Create(context.TODO(), giteaCustomResource); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Custom Resource", giteaCustomResource.Name)
	}

	// Wait for server to be running
	if !kubernetes.GetK8Client().GetDeploymentStatus("gitea-server", giteaNamespace.Name) {
		return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, nil
	}

	// Extract app route suffix from openshift-console
	giteaRouteFound := &routev1.Route{}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: "gitea-server", Namespace: giteaNamespace.Name}, giteaRouteFound); err != nil {
		log.Errorf("Failed to find %s route", "gitea-server")
		return reconcile.Result{}, err
	}

	giteaURL := "https://" + giteaRouteFound.Spec.Host

	// Create workshop users in gitea
	for id := 1; id <= users; id++ {
		username := fmt.Sprintf("user%d", id)
		if result, err := createGitUser(workshop, username, giteaURL); err != nil {
			return result, err
		}
	}

	//Success
	return reconcile.Result{}, nil
}

// Create GitUser
func createGitUser(workshop *workshopv1.Workshop, username string, giteaURL string) (reconcile.Result, error) {

	var (
		openshiftUserPassword = workshop.Spec.User.Password
		err                   error
		httpResponse          *http.Response
		httpRequest           *http.Request
		requestURL            = giteaURL + "/user/sign_up"
		body                  = url.Values{}
		client                = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			// Do not follow Redirect
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	)

	body.Set("user_name", username)
	body.Set("email", username+"@none.com")
	body.Set("password", openshiftUserPassword)
	body.Set("retype", openshiftUserPassword)

	httpRequest, err = http.NewRequest("POST", requestURL, strings.NewReader(body.Encode()))
	if err != nil {
		log.Error(err, "Failed http POST Request  ")
	}
	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("Content-Length", strconv.Itoa(len(body.Encode())))

	httpResponse, err = client.Do(httpRequest)
	if err != nil {
		return reconcile.Result{}, err
	}
	if httpResponse.StatusCode == http.StatusCreated {
		log.Infof("Created %s user in Gitea", username)
	}

	defer httpResponse.Body.Close()

	//Success
	return reconcile.Result{}, nil
}

// Delete Gitea
func (r *WorkshopReconciler) deleteGitea(workshop *workshopv1.Workshop, users int, giteaNamespaceName string) (reconcile.Result, error) {

	imageName := workshop.Spec.Infrastructure.Gitea.Image.Name
	imageTag := workshop.Spec.Infrastructure.Gitea.Image.Tag

	labels := map[string]string{
		"app":                       "gitea",
		"app.kubernetes.io/name":    "gitea",
		"app.kubernetes.io/part-of": "gitea",
	}

	giteaNamespace := kubernetes.NewNamespace(workshop, r.Scheme, giteaNamespaceName)

	giteaCustomResource := gitea.NewCustomResource(workshop, r.Scheme, "gitea-server", giteaNamespace.Name, labels)
	giteaCustomResourceFound := &gitea.Gitea{}
	giteaCustomResourceErr := r.Get(context.TODO(), types.NamespacedName{Name: giteaCustomResource.Name, Namespace: giteaNamespace.Name},giteaCustomResourceFound )
	if giteaCustomResourceErr == nil {
		// Delete Custom Resource
		if err := r.Delete(context.TODO(), giteaCustomResource); err != nil{
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s gitea Custom Resource", giteaCustomResource.Name)
	}

	giteaOperator := kubernetes.NewAnsibleOperatorDeployment(workshop, r.Scheme, "gitea-operator", giteaNamespace.Name, labels, imageName+":"+imageTag, "gitea-operator")
	giteaOperatorFound := &appsv1.Deployment{}
	giteaOperatorErr := r.Get(context.TODO(), types.NamespacedName{Name: giteaOperator.Name ,Namespace: giteaNamespace.Name},giteaOperatorFound )
	if giteaOperatorErr == nil {
		// Delete Operator
		if err := r.Delete(context.TODO(), giteaOperator); err != nil{
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s gitea Operator", giteaOperator.Name)
	}


	giteaClusterRoleBinding := kubernetes.NewClusterRoleBindingSA(workshop, r.Scheme, "gitea-operator", giteaNamespace.Name, labels, "gitea-operator", "gitea-operator", "ClusterRole")
	giteaClusterRoleBindingFound := &rbac.ClusterRoleBinding{}
	giteaClusterRoleBindingErr := r.Get(context.TODO(), types.NamespacedName{Name:giteaClusterRoleBinding.Name,Namespace: giteaNamespace.Name},giteaClusterRoleBindingFound )
	if giteaClusterRoleBindingErr == nil {
		// Delete Cluster Role Binding
		if err := r.Delete(context.TODO(), giteaClusterRoleBinding) ; err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s gitea Cluster  Role Binding", giteaClusterRoleBinding.Name)
	}

	giteaClusterRole := kubernetes.NewClusterRole(workshop, r.Scheme, "gitea-operator", giteaNamespace.Name, labels, kubernetes.GiteaRules())
	giteaClusterRoleFound :=&rbac.ClusterRole{}
	giteaClusterRoleErr := r.Get(context.TODO(), types.NamespacedName{Name:giteaClusterRole.Name,Namespace: giteaNamespace.Name}, giteaClusterRoleFound)
	if giteaClusterRoleErr == nil {
		// Delete Cluster Role
		if err := r.Delete(context.TODO(), giteaClusterRole); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s gitea Cluster Role", giteaClusterRole.Name)
	}


	giteaServiceAccount := kubernetes.NewServiceAccount(workshop, r.Scheme, "gitea-operator", giteaNamespace.Name, labels)
	giteaServiceAccountFound := &corev1.ServiceAccount{}
	giteaServiceAccountErr := r.Get(context.TODO(), types.NamespacedName{Name: giteaServiceAccount.Name, Namespace: giteaNamespace.Name}, giteaServiceAccountFound)
	if giteaServiceAccountErr == nil {
		// Delete Service Account
		if err := r.Delete(context.TODO(), giteaServiceAccount); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s gitea Service Account", giteaServiceAccount.Name)
	}

	giteaCustomResourceDefinition := kubernetes.NewCustomResourceDefinition(workshop, r.Scheme, "giteas.gpte.opentlc.com", "gpte.opentlc.com", "Gitea", "GiteaList", "giteas", "gitea", "v1alpha1", nil, nil)
	giteaCustomResourceDefinitionFound := &apiextensionsv1beta1.CustomResourceDefinition{}
	giteaCustomResourceDefinitionErr := r.Get(context.TODO(), types.NamespacedName{Name: giteaCustomResourceDefinition.Name}, giteaCustomResourceDefinitionFound)
	if giteaCustomResourceDefinitionErr == nil {
		// Delete CRD
		if err := r.Delete(context.TODO(), giteaCustomResourceDefinition); err != nil{
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s gitea Custom Resource Definition", giteaCustomResourceDefinition.Name)
	}

	giteaNamespaceFound := &corev1.Namespace{}
	giteaNamespaceErr := r.Get(context.TODO(), types.NamespacedName{Name: giteaNamespace.Name}, giteaNamespaceFound)
	if giteaNamespaceErr == nil {
		// Delete Project
		if err := r.Delete(context.TODO(), giteaNamespace); err != nil{
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s gitea Project ", giteaNamespace.Name)
	}

	//Success
	return reconcile.Result{}, nil
}
