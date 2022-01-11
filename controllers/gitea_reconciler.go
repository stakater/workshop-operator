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

var gitealabels = map[string]string{
	"app":                       "gitea",
	"app.kubernetes.io/name":    "gitea",
	"app.kubernetes.io/part-of": "gitea",
}

const (
	GITEANAMESPACENAME         = "gitea"
	GITEADEPLOYMENTNAME        = "gitea-server"
	GITEAANSIBLEDEPLOYMENTNAME = "gitea-operator"
	CLUSTERROLEKINDNAME        = "ClusterRole"
	GITEACRDNAME               = "giteas.gpte.opentlc.com"
	GITEACRDGROUPNAME          = "gpte.opentlc.com"
	GITEACRDKINDNAME           = "Gitea"
	GITEACRDLISTKINDNAME       = "GiteaList"
	GITEACRDPLURALNAME         = "giteas"
	GITEACRDSINGULARNAME       = "gitea"
	GITEACRDVERSIONAME         = "v1alpha1"
	GITEACRNAME                = "gitea-server"
	GITEAROLEBINDINGNAME       = "gitea-operator"
	GITEASERVICEACCOUNTNAME    = "gitea-operator"
	GITEACLUSTERROLENAME       = "gitea-operator"
)

// Reconciling Gitea
func (r *WorkshopReconciler) reconcileGitea(workshop *workshopv1.Workshop, users int) (reconcile.Result, error) {
	enabledGitea := workshop.Spec.Infrastructure.Gitea.Enabled

	if enabledGitea {
		if result, err := r.addGitea(workshop, users); util.IsRequeued(result, err) {
			return result, err
		}
	}

	//Success
	return reconcile.Result{}, nil
}

// Add Gitea
func (r *WorkshopReconciler) addGitea(workshop *workshopv1.Workshop, users int) (reconcile.Result, error) {

	imageName := workshop.Spec.Infrastructure.Gitea.Image.Name
	imageTag := workshop.Spec.Infrastructure.Gitea.Image.Tag

	// Create Project
	giteaNamespace := kubernetes.NewNamespace(workshop, r.Scheme, GITEANAMESPACENAME)
	if err := r.Create(context.TODO(), giteaNamespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Project", giteaNamespace.Name)
	}

	// Create CRD
	giteaCustomResourceDefinition := kubernetes.NewCustomResourceDefinition(workshop, r.Scheme, GITEACRDNAME, GITEACRDGROUPNAME, GITEACRDKINDNAME, GITEACRDLISTKINDNAME, GITEACRDPLURALNAME, GITEACRDSINGULARNAME, GITEACRDVERSIONAME, nil, nil)
	if err := r.Create(context.TODO(), giteaCustomResourceDefinition); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Custom Resource Definition", giteaCustomResourceDefinition.Name)
	}

	// Create Service Account
	giteaServiceAccount := kubernetes.NewServiceAccount(workshop, r.Scheme, GITEASERVICEACCOUNTNAME, giteaNamespace.Name, gitealabels)
	if err := r.Create(context.TODO(), giteaServiceAccount); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Service Account", giteaServiceAccount.Name)
	}

	// Create Cluster Role
	giteaClusterRole := kubernetes.NewClusterRole(workshop, r.Scheme, GITEACLUSTERROLENAME, giteaNamespace.Name, gitealabels, kubernetes.GiteaRules())
	if err := r.Create(context.TODO(), giteaClusterRole); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Cluster Role", giteaClusterRole.Name)
	}

	// Create Cluster Role Binding
	giteaClusterRoleBinding := kubernetes.NewClusterRoleBindingSA(workshop, r.Scheme, GITEAROLEBINDINGNAME, giteaNamespace.Name, gitealabels, GITEASERVICEACCOUNTNAME, GITEAROLEBINDINGNAME, CLUSTERROLEKINDNAME)
	if err := r.Create(context.TODO(), giteaClusterRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Cluster Role Binding", giteaClusterRoleBinding.Name)
	}

	giteaOperator := kubernetes.NewAnsibleOperatorDeployment(workshop, r.Scheme, GITEAANSIBLEDEPLOYMENTNAME, giteaNamespace.Name, gitealabels, imageName+":"+imageTag, GITEASERVICEACCOUNTNAME)

	// Create Operator
	if err := r.Create(context.TODO(), giteaOperator); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Operator", giteaOperator.Name)
	}

	// Create Custom Resource
	giteaCustomResource := gitea.NewCustomResource(workshop, r.Scheme, GITEACRNAME, giteaNamespace.Name, gitealabels)
	if err := r.Create(context.TODO(), giteaCustomResource); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Custom Resource", giteaCustomResource.Name)
	}

	// Wait for server to be running
	if !kubernetes.GetK8Client().GetDeploymentStatus(GITEADEPLOYMENTNAME, giteaNamespace.Name) {
		return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, nil
	}

	// Extract app route suffix from openshift-console
	giteaRouteFound := &routev1.Route{}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: GITEADEPLOYMENTNAME, Namespace: giteaNamespace.Name}, giteaRouteFound); err != nil {
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
		openshiftUserPassword = workshop.Spec.UserDetails.DefaultPassword
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
func (r *WorkshopReconciler) deleteGitea(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	log.Info("Deleting gitea")

	imageName := workshop.Spec.Infrastructure.Gitea.Image.Name
	imageTag := workshop.Spec.Infrastructure.Gitea.Image.Tag

	giteaCustomResource := gitea.NewCustomResource(workshop, r.Scheme, GITEACRNAME, GITEANAMESPACENAME, gitealabels)
	// Delete Custom Resource
	if err := r.Delete(context.TODO(), giteaCustomResource); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s gitea Custom Resource", giteaCustomResource.Name)

	giteaOperator := kubernetes.NewAnsibleOperatorDeployment(workshop, r.Scheme, GITEAANSIBLEDEPLOYMENTNAME, GITEANAMESPACENAME, gitealabels, imageName+":"+imageTag, GITEASERVICEACCOUNTNAME)
	// Delete Operator
	if err := r.Delete(context.TODO(), giteaOperator); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s gitea Operator", giteaOperator.Name)

	giteaClusterRoleBinding := kubernetes.NewClusterRoleBindingSA(workshop, r.Scheme, GITEAROLEBINDINGNAME, GITEANAMESPACENAME, gitealabels, GITEASERVICEACCOUNTNAME, GITEACLUSTERROLENAME, CLUSTERROLEKINDNAME)
	// Delete Cluster Role Binding
	if err := r.Delete(context.TODO(), giteaClusterRoleBinding); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s gitea Cluster  Role Binding", giteaClusterRoleBinding.Name)

	giteaClusterRole := kubernetes.NewClusterRole(workshop, r.Scheme, GITEACLUSTERROLENAME, GITEANAMESPACENAME, gitealabels, kubernetes.GiteaRules())
	// Delete Cluster Role
	if err := r.Delete(context.TODO(), giteaClusterRole); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s gitea Cluster Role", giteaClusterRole.Name)

	giteaServiceAccount := kubernetes.NewServiceAccount(workshop, r.Scheme, GITEASERVICEACCOUNTNAME, GITEANAMESPACENAME, gitealabels)
	// Delete Service Account
	if err := r.Delete(context.TODO(), giteaServiceAccount); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s gitea Service Account", giteaServiceAccount.Name)

	giteaCustomResourceDefinition := kubernetes.NewCustomResourceDefinition(workshop, r.Scheme, GITEACRDNAME, GITEACRDGROUPNAME, GITEACRDKINDNAME, GITEACRDLISTKINDNAME, GITEACRDPLURALNAME, GITEACRDSINGULARNAME, GITEACRDVERSIONAME, nil, nil)
	// Delete CRD
	if err := r.Delete(context.TODO(), giteaCustomResourceDefinition); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s gitea Custom Resource Definition", giteaCustomResourceDefinition.Name)

	giteaNamespace := kubernetes.NewNamespace(workshop, r.Scheme, GITEANAMESPACENAME)
	// Delete Project
	if err := r.Delete(context.TODO(), giteaNamespace); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s gitea Project ", GITEANAMESPACENAME)
	log.Info("Gitea deleted succesfully")

	//Success
	return reconcile.Result{}, nil
}
