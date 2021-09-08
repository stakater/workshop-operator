package controllers

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	"github.com/stakater/workshop-operator/common/gitea"
	"github.com/stakater/workshop-operator/common/kubernetes"

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

// TODO: Delete Gitea
