package controllers

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	_ "k8s.io/api/rbac/v1"

	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	"github.com/stakater/workshop-operator/common/codeready"
	"github.com/stakater/workshop-operator/common/kubernetes"
	"github.com/stakater/workshop-operator/common/util"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"
)

var codereadylabels = map[string]string{
	"app.kubernetes.io/part-of": "codeready",
}

const (
	CODEREADYNAMESPACENAME          = "workspaces"
	CODEREADYOPERATORGROUPNAME      = "codeready-workspaces"
	CODEREADYREDHATSUBSCRIPTIONNAME = "codeready-workspaces"
	CODEREADYPACKAGENAME            = "codeready-workspaces"
	CODEREADYDEPLOYMENTNAME         = "codeready-operator"
	CODEREADYCUSTOMRESOURCENAME     = "codereadyworkspaces"
	CODEREADYDEPLOYMENTSTATUSNAME   = "codeready"
	CODEREADYCLUSTERROLENAME        = "che"
	CODEREADYCLUSTERROLEBINDINGNAME = "che"
	CODEREADYSERVICEACCOUNTNAME     = "che"
	CODEREADYCLUSTERROLEKINDNAME    = "ClusterRole"
	CODEREADYUSERCODEFLAVORNAME     = "codeready"
)

// Reconciling CodeReadyWorkspace
func (r *WorkshopReconciler) reconcileCodeReadyWorkspace(workshop *workshopv1.Workshop, users int,
	appsHostnameSuffix string, openshiftConsoleURL string) (reconcile.Result, error) {
	log.Infoln("Creating CodeReadyWorkspace")
	enabled := workshop.Spec.Infrastructure.CodeReadyWorkspace.Enabled

	if enabled {
		if result, err := r.addCodeReadyWorkspace(workshop, users, appsHostnameSuffix); util.IsRequeued(result, err) {
			return result, err
		}
	}

	//Success
	return reconcile.Result{}, nil
}

func (r *WorkshopReconciler) addCodeReadyWorkspace(workshop *workshopv1.Workshop, users int,
	appsHostnameSuffix string) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.CodeReadyWorkspace.OperatorHub.Channel
	clusterServiceVersion := workshop.Spec.Infrastructure.CodeReadyWorkspace.OperatorHub.ClusterServiceVersion

	// Create Project
	codeReadyWorkspacesNamespace := kubernetes.NewNamespace(workshop, r.Scheme, CODEREADYNAMESPACENAME)
	if err := r.Create(context.TODO(), codeReadyWorkspacesNamespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s CodeReadyWorkspace Project", codeReadyWorkspacesNamespace.Name)
	}

	// Create OperatorGroup
	codeReadyWorkspacesOperatorGroup := kubernetes.NewOperatorGroup(workshop, r.Scheme, CODEREADYOPERATORGROUPNAME, CODEREADYNAMESPACENAME)
	if err := r.Create(context.TODO(), codeReadyWorkspacesOperatorGroup); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s CodeReadyWorkspace OperatorGroup", codeReadyWorkspacesOperatorGroup.Name)
	}

	// Create Subscription
	codeReadyWorkspacesSubscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, CODEREADYREDHATSUBSCRIPTIONNAME, CODEREADYNAMESPACENAME,
		CODEREADYPACKAGENAME, channel, clusterServiceVersion)
	if err := r.Create(context.TODO(), codeReadyWorkspacesSubscription); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s CodeReadyWorkspace Subscription", codeReadyWorkspacesSubscription.Name)
	}

	// Approve the Installation
	if err := r.ApproveInstallPlan(clusterServiceVersion, CODEREADYREDHATSUBSCRIPTIONNAME, CODEREADYNAMESPACENAME); err != nil {
		log.Warnf("Waiting for CodeReadyWorkspace Subscription to create InstallPlan for %s", "codeready-workspaces")
		return reconcile.Result{Requeue: true}, nil
	}

	// Wait for CodeReadyWorkspace Operator to be running
	if !kubernetes.GetK8Client().GetDeploymentStatus(CODEREADYDEPLOYMENTNAME, CODEREADYNAMESPACENAME) {
		return reconcile.Result{Requeue: true}, nil
	}

	codeReadyWorkspacesCustomResource := codeready.NewCustomResource(workshop, r.Scheme, CODEREADYCUSTOMRESOURCENAME, CODEREADYNAMESPACENAME)
	if err := r.Create(context.TODO(), codeReadyWorkspacesCustomResource); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s CodeReadyWorkspace Custom Resource", codeReadyWorkspacesCustomResource.Name)
	}

	// Wait for CodeReadyWorkspace to be running
	if !kubernetes.GetK8Client().GetDeploymentStatus(CODEREADYDEPLOYMENTSTATUSNAME, CODEREADYNAMESPACENAME) {
		return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 1}, nil
	}

	// Initialize Workspaces from devfile
	devfile, result, err := getDevFile(workshop)
	if err != nil {
		return result, err
	}

	// Users and Workspaces
	if !workshop.Spec.Infrastructure.CodeReadyWorkspace.OpenshiftOAuth {
		masterAccessToken, result, err := getKeycloakAdminToken(workshop, CODEREADYNAMESPACENAME, appsHostnameSuffix)
		if err != nil {
			return result, err
		}

		// Che Cluster Role
		cheClusterRole :=
			kubernetes.NewClusterRole(workshop, r.Scheme, CODEREADYCLUSTERROLENAME, CODEREADYNAMESPACENAME, codereadylabels, kubernetes.CheRules())
		if err := r.Create(context.TODO(), cheClusterRole); err != nil && !errors.IsAlreadyExists(err) {
			return reconcile.Result{}, err
		} else if err == nil {
			log.Infof("Created %s CodeReadyWorkspace Cluster Role", cheClusterRole.Name)
		}

		// Che Cluster Role Binding
		cheClusterRoleBinding := kubernetes.NewClusterRoleBindingSA(workshop, r.Scheme, CODEREADYCLUSTERROLEBINDINGNAME, CODEREADYNAMESPACENAME, codereadylabels, CODEREADYSERVICEACCOUNTNAME, cheClusterRole.Name, CODEREADYCLUSTERROLEKINDNAME)
		if err := r.Create(context.TODO(), cheClusterRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
			return reconcile.Result{}, err
		} else if err == nil {
			log.Infof("Created %s CodeReadyWorkspace Cluster Role Binding", cheClusterRoleBinding.Name)
		}

		for id := 1; id <= users; id++ {
			username := fmt.Sprintf("user%d", id)

			if result, err := createUser(workshop, username, CODEREADYUSERCODEFLAVORNAME, CODEREADYNAMESPACENAME, appsHostnameSuffix, masterAccessToken); err != nil {
				return result, err
			}

			userAccessToken, result, err := getUserToken(workshop, username, CODEREADYUSERCODEFLAVORNAME, CODEREADYNAMESPACENAME, appsHostnameSuffix)
			if err != nil {
				return result, err
			}

			if result, err := initWorkspace(workshop, username, CODEREADYUSERCODEFLAVORNAME, CODEREADYNAMESPACENAME, userAccessToken, devfile, appsHostnameSuffix); err != nil {
				return result, err
			}

		}
	} else {
		for id := 1; id <= users; id++ {
			username := fmt.Sprintf("user%d", id)

			userAccessToken, result, err := getOAuthUserToken(workshop, username, CODEREADYUSERCODEFLAVORNAME, CODEREADYNAMESPACENAME, appsHostnameSuffix)
			if err != nil {
				return result, err
			}

			if result, err := updateUserEmail(workshop, username, CODEREADYUSERCODEFLAVORNAME, CODEREADYNAMESPACENAME, appsHostnameSuffix); err != nil {
				return result, err
			}

			if result, err := initWorkspace(workshop, username, CODEREADYUSERCODEFLAVORNAME, CODEREADYNAMESPACENAME, userAccessToken, devfile, appsHostnameSuffix); err != nil {
				return result, err
			}
		}
	}

	//Success
	return reconcile.Result{}, nil
}

// Get DevFile
func getDevFile(workshop *workshopv1.Workshop) (string, reconcile.Result, error) {

	var (
		httpResponse *http.Response
		httpRequest  *http.Request
		devfile      string
		client       = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			// Do not follow Redirect
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	)

	gitURL, err := url.Parse(workshop.Spec.Source.GitURL)
	if err != nil {
		return "", reconcile.Result{}, err
	}
	devfileRawURL := fmt.Sprintf("https://raw.githubusercontent.com%s/%s/devfile.yaml", gitURL.Path, workshop.Spec.Source.GitBranch)
	httpRequest, err = http.NewRequest("GET", devfileRawURL, nil)
	if err != nil {
		log.Error(err, "Failed http GET Request")
	}
	httpResponse, err = client.Do(httpRequest)
	if err != nil {
		log.Errorf("Error when getting Devfile from %s", devfileRawURL)
		return "", reconcile.Result{}, err
	}

	if httpResponse.StatusCode == http.StatusOK {
		bodyBytes, err := ioutil.ReadAll(httpResponse.Body)
		if err != nil {
			log.Errorf("Error when reading %s", devfileRawURL)
			return "", reconcile.Result{}, err
		}

		bodyJSON, err := yaml.YAMLToJSON(bodyBytes)
		if err != nil {
			log.Errorf("Error to converting %s to JSON", devfileRawURL)
			return "", reconcile.Result{}, err
		}
		devfile = string(bodyJSON)
	} else {
		log.Errorf("Error (%v) when getting Devfile from %s", httpResponse.StatusCode, devfileRawURL)
		return "", reconcile.Result{}, err
	}

	return devfile, reconcile.Result{}, nil
}

// Create user
func createUser(workshop *workshopv1.Workshop, username string, codeflavor string,
	namespace string, appsHostnameSuffix string, masterToken string) (reconcile.Result, error) {

	var (
		openshiftUserPassword = workshop.Spec.User.Password
		body                  []byte
		err                   error
		httpResponse          *http.Response
		httpRequest           *http.Request
		keycloakCheUserURL    = "https://keycloak-" + namespace + "." + appsHostnameSuffix + "/auth/admin/realms/" + codeflavor + "/users"

		client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			// Do not follow Redirect
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	)

	body, err = json.Marshal(codeready.NewUser(username, openshiftUserPassword))
	if err != nil {
		return reconcile.Result{}, err
	}

	httpRequest, err = http.NewRequest("POST", keycloakCheUserURL, bytes.NewBuffer(body))
	if err != nil {
		log.Error(err, "Failed http POST Request")
	}
	httpRequest.Header.Set("Authorization", "Bearer "+masterToken)
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, err = client.Do(httpRequest)
	if err != nil {
		return reconcile.Result{}, err
	}
	if httpResponse.StatusCode == http.StatusCreated {
		log.Infof("Created %s in CodeReady Workspaces", username)
	}

	return reconcile.Result{}, nil
}

// Get user token
func getUserToken(workshop *workshopv1.Workshop, username string, codeflavor string, namespace string, appsHostnameSuffix string) (string, reconcile.Result, error) {

	var (
		openshiftUserPassword = workshop.Spec.User.Password
		err                   error
		httpResponse          *http.Response
		httpRequest           *http.Request
		keycloakCheTokenURL   = "https://keycloak-" + namespace + "." + appsHostnameSuffix + "/auth/realms/" + codeflavor + "/protocol/openid-connect/token"

		userToken util.Token
		client    = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			// Do not follow Redirect
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	)

	// Get User Access Token
	data := url.Values{}
	data.Set("username", username)
	data.Set("password", openshiftUserPassword)
	data.Set("client_id", codeflavor+"-public")
	data.Set("grant_type", "password")

	httpRequest, err = http.NewRequest("POST", keycloakCheTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		log.Error(err, "Failed http POST Request")
	}
	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpResponse, err = client.Do(httpRequest)
	if err != nil {
		log.Errorf("Error to get the user access  token from %s keycloak (%v)", codeflavor, err)
		return "", reconcile.Result{}, err
	}

	defer httpResponse.Body.Close()

	if httpResponse.StatusCode == http.StatusOK {
		if err := json.NewDecoder(httpResponse.Body).Decode(&userToken); err != nil {
			log.Errorf("Error to get the user access  token from %s keycloak (%v)", codeflavor, err)
			return "", reconcile.Result{}, err
		}
	} else {
		log.Errorf("Error to get the user access token from %s keycloak (%d)", codeflavor, httpResponse.StatusCode)
		return "", reconcile.Result{}, err
	}

	return userToken.AccessToken, reconcile.Result{}, nil
}

// Get oauthUserToken
func getOAuthUserToken(workshop *workshopv1.Workshop, username string,
	codeflavor string, namespace string, appsHostnameSuffix string) (string, reconcile.Result, error) {
	var (
		openshiftUserPassword = workshop.Spec.User.Password
		err                   error
		httpResponse          *http.Response
		httpRequest           *http.Request
		keycloakCheTokenURL   = "https://keycloak-" + namespace + "." + appsHostnameSuffix + "/auth/realms/" + codeflavor + "/protocol/openid-connect/token"
		oauthOpenShiftURL     = "https://oauth-openshift." + appsHostnameSuffix + "/oauth/authorize?client_id=openshift-challenging-client&response_type=token"

		userToken util.Token
		client    = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			// Do not follow Redirect
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	)

	// GET TOKEN
	httpRequest, err = http.NewRequest("GET", oauthOpenShiftURL, nil)
	if err != nil {
		log.Error(err, "Failed http GET Request")
	}
	httpRequest.Header.Set("Authorization", "Basic "+util.GetBasicAuth(username, openshiftUserPassword))
	httpRequest.Header.Set("X-CSRF-Token", "xxx")

	httpResponse, err = client.Do(httpRequest)
	if err != nil {
		log.Errorf("Error when getting Token Exchange for %s: %v", username, err)
		return "", reconcile.Result{}, err
	}

	if httpResponse.StatusCode == http.StatusFound {
		locationURL, err := url.Parse(httpResponse.Header.Get("Location"))
		if err != nil {
			return "", reconcile.Result{}, err
		}

		regex := regexp.MustCompile("access_token=([^&]+)")
		subjectToken := regex.FindStringSubmatch(locationURL.Fragment)

		// Get User Access Token
		data := url.Values{}
		data.Set("client_id", codeflavor+"-public")
		data.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
		data.Set("subject_token", subjectToken[1])
		data.Set("subject_issuer", "openshift-v4")
		data.Set("subject_token_type", "urn:ietf:params:oauth:token-type:access_token")

		httpRequest, err = http.NewRequest("POST", keycloakCheTokenURL, strings.NewReader(data.Encode()))
		if err != nil {
			log.Error(err, "Failed http POST Request")
		}
		httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		httpResponse, err = client.Do(httpRequest)
		if err != nil {
			log.Errorf("Error to get the oauth user access  token from %s keycloak (%v)", codeflavor, err)
			return "", reconcile.Result{}, err
		}
		defer httpResponse.Body.Close()
		if httpResponse.StatusCode == http.StatusOK {
			if err := json.NewDecoder(httpResponse.Body).Decode(&userToken); err != nil {
				log.Errorf("Error to get the oauth user access  token from %s keycloak (%v)", codeflavor, err)
				return "", reconcile.Result{}, err
			}
		} else {
			log.Errorf("Error to get the oauth user access token from %s keycloak (%d)", codeflavor, httpResponse.StatusCode)
			return "", reconcile.Result{}, err
		}
	} else {
		log.Errorf("Error when getting Token Exchange for %s (%d)", username, httpResponse.StatusCode)
		return "", reconcile.Result{}, err
	}

	return userToken.AccessToken, reconcile.Result{}, nil
}

// Get KeyCloak Admin Token
func getKeycloakAdminToken(workshop *workshopv1.Workshop, namespace string, appsHostnameSuffix string) (string, reconcile.Result, error) {
	var (
		err                 error
		httpResponse        *http.Response
		httpRequest         *http.Request
		keycloakCheTokenURL = "https://keycloak-" + namespace + "." + appsHostnameSuffix + "/auth/realms/master/protocol/openid-connect/token"

		masterToken util.Token
		client      = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			// Do not follow Redirect
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	)

	// GET TOKEN
	httpRequest, err = http.NewRequest("POST", keycloakCheTokenURL, strings.NewReader("username=admin&password=admin&grant_type=password&client_id=admin-cli"))
	if err != nil {
		log.Error(err, "Failed http POST Request")
	}
	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpResponse, err = client.Do(httpRequest)
	if err != nil {
		return "", reconcile.Result{}, err
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode == http.StatusOK {
		if err := json.NewDecoder(httpResponse.Body).Decode(&masterToken); err != nil {
			return "", reconcile.Result{}, err
		}
	}

	return masterToken.AccessToken, reconcile.Result{}, nil
}

// Update User Email
func updateUserEmail(workshop *workshopv1.Workshop, username string,
	codeflavor string, namespace string, appsHostnameSuffix string) (reconcile.Result, error) {
	var (
		err                    error
		httpResponse           *http.Response
		httpRequest            *http.Request
		keycloakMasterTokenURL = "https://keycloak-" + namespace + "." + appsHostnameSuffix + "/auth/realms/master/protocol/openid-connect/token"
		keycloakUserURL        = "https://keycloak-" + namespace + "." + appsHostnameSuffix + "/auth/admin/realms/" + codeflavor + "/users"
		masterToken            util.Token
		client                 = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			// Do not follow Redirect
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		cheUser []struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		}
	)

	// Get Keycloak Admin Token
	httpRequest, err = http.NewRequest("POST", keycloakMasterTokenURL, strings.NewReader("username=admin&password=admin&grant_type=password&client_id=admin-cli"))
	if err != nil {
		log.Error(err, "Failed http POST Request")
	}
	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpResponse, err = client.Do(httpRequest)
	if err != nil {
		log.Errorf("Error when getting the master token from %s keycloak (%v)", codeflavor, err)
		return reconcile.Result{}, err
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode == http.StatusOK {
		if err := json.NewDecoder(httpResponse.Body).Decode(&masterToken); err != nil {
			log.Errorf("Error when reading the master token: %v", err)
			return reconcile.Result{}, err
		}
	} else {
		log.Errorf("Error when getting the master token from %s keycloak (%d)", codeflavor, httpResponse.StatusCode)
		return reconcile.Result{}, err
	}

	// GET USER
	httpRequest, err = http.NewRequest("GET", keycloakUserURL+"?username="+username, nil)
	if err != nil {
		log.Error(err, "Failed http GET Request")
	}
	httpRequest.Header.Set("Authorization", "Bearer "+masterToken.AccessToken)

	httpResponse, err = client.Do(httpRequest)
	if err != nil {
		log.Errorf("Error when getting %s user: %v", username, err)
		return reconcile.Result{}, err
	}

	defer httpResponse.Body.Close()
	if httpResponse.StatusCode == http.StatusOK {
		if err := json.NewDecoder(httpResponse.Body).Decode(&cheUser); err != nil {
			log.Errorf("Error to get the user info (%v)", err)
			return reconcile.Result{}, err
		}

		if cheUser[0].Email == "" {
			httpRequest, err = http.NewRequest("PUT", keycloakUserURL+"/"+cheUser[0].ID,
				strings.NewReader(`{"email":"`+username+`@none.com"}`))
			if err != nil {
				log.Error(err, "Failed http PUT Request")
			}
			httpRequest.Header.Set("Content-Type", "application/json")
			httpRequest.Header.Set("Authorization", "Bearer "+masterToken.AccessToken)

			// remove httpResponse because it is unused
			_, err = client.Do(httpRequest)

			if err != nil {
				log.Errorf("Error when update email address for %s: %v", username, err)
				return reconcile.Result{}, err
			}
		}
	} else {
		log.Errorf("Error when getting %s user: %v", username, httpResponse.StatusCode)
		return reconcile.Result{}, err
	}

	//Success
	return reconcile.Result{}, nil
}

// Initialize workspace
func initWorkspace(workshop *workshopv1.Workshop, username string,
	codeflavor string, namespace string, userAccessToken string, devfile string,
	appsHostnameSuffix string) (reconcile.Result, error) {

	var (
		err                 error
		httpResponse        *http.Response
		httpRequest         *http.Request
		devfileWorkspaceURL = "https://" + codeflavor + "-" + namespace + "." + appsHostnameSuffix + "/api/workspace/devfile?start-after-create=true&namespace=" + username

		client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			// Do not follow Redirect
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	)

	httpRequest, err = http.NewRequest("POST", devfileWorkspaceURL, strings.NewReader(devfile))
	if err != nil {
		log.Error(err, "Failed http POST Request")
	}
	httpRequest.Header.Set("Authorization", "Bearer "+userAccessToken)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")

	httpResponse, err = client.Do(httpRequest)
	if err != nil {
		log.Errorf("Error when creating the workspace for %s: %v", username, err)
		return reconcile.Result{}, err
	}
	defer httpResponse.Body.Close()

	//Success
	return reconcile.Result{}, nil
}

func (r *WorkshopReconciler) deleteCodeReadyWorkspace(workshop *workshopv1.Workshop, users int,
	appsHostnameSuffix string, openshiftConsoleURL string) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.CodeReadyWorkspace.OperatorHub.Channel
	clusterServiceVersion := workshop.Spec.Infrastructure.CodeReadyWorkspace.OperatorHub.ClusterServiceVersion

	cheClusterRole :=
		kubernetes.NewClusterRole(workshop, r.Scheme, CODEREADYCLUSTERROLENAME, CODEREADYNAMESPACENAME, codereadylabels, kubernetes.CheRules())

	cheClusterRoleBinding := kubernetes.NewClusterRoleBindingSA(workshop, r.Scheme, CODEREADYCLUSTERROLEBINDINGNAME, CODEREADYNAMESPACENAME, codereadylabels, CODEREADYSERVICEACCOUNTNAME, cheClusterRole.Name, CODEREADYCLUSTERROLEKINDNAME)
	// Delete che Cluster RoleBinding
	if err := r.Delete(context.TODO(), cheClusterRoleBinding); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s CodeReadyWorkspace Cluster RoleBinding ", cheClusterRoleBinding.Name)

	// Delete che Cluster Role
	if err := r.Delete(context.TODO(), cheClusterRole); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s CodeReadyWorkspace Cluster Role ", cheClusterRole.Name)

	codeReadyWorkspacesCustomResource := codeready.NewCustomResource(workshop, r.Scheme, CODEREADYCUSTOMRESOURCENAME, CODEREADYNAMESPACENAME)
	// Delete codeReadyWorkspaces CustomResource
	if err := r.Delete(context.TODO(), codeReadyWorkspacesCustomResource); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s codeReadyWorkspaces CustomResource", codeReadyWorkspacesCustomResource.Name)

	codeReadyWorkspacesSubscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, CODEREADYREDHATSUBSCRIPTIONNAME, CODEREADYNAMESPACENAME,
		CODEREADYPACKAGENAME, channel, clusterServiceVersion)
	// Delete Subscription
	if err := r.Delete(context.TODO(), codeReadyWorkspacesSubscription); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s codeReadyWorkspaces Subscription", codeReadyWorkspacesSubscription.Name)

	codeReadyWorkspacesOperatorGroup := kubernetes.NewOperatorGroup(workshop, r.Scheme, CODEREADYOPERATORGROUPNAME, CODEREADYNAMESPACENAME)
	// Delete OperatorGroup
	if err := r.Delete(context.TODO(), codeReadyWorkspacesOperatorGroup); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s codeReadyWorkspaces OperatorGroup", codeReadyWorkspacesOperatorGroup.Name)

	codeReadyWorkspacesNamespace := kubernetes.NewNamespace(workshop, r.Scheme, CODEREADYNAMESPACENAME)
	// Delete Project
	if err := r.Delete(context.TODO(), codeReadyWorkspacesNamespace); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s codeReadyWorkspaces namespace", codeReadyWorkspacesNamespace.Name)

	//Success
	return reconcile.Result{}, nil
}
