package controllers

import (
	"context"
	"fmt"
	"reflect"

	argocdoperatorv1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	argocdv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	"github.com/stakater/workshop-operator/common/argocd"
	"github.com/stakater/workshop-operator/common/kubernetes"
	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"

	"github.com/stakater/workshop-operator/common/util"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	secretName              = "argocd-default-cluster-config"
	clusterConfigSecretData = map[string]string{}
)

const (
	ARGOCDNAMESPACENAME            = "argocd"
	GITOPSNAME                     = "openshift-gitops-operator"
	GITOPSOPERATORNAMESPACENAME    = "openshift-operators"
	GITOPSDEPLOYMENTSTATUSNAME     = "gitops-operator"
	GITOPSROLENAME                 = "argocd-manager"
	GITOPSROLEBINDINGNAME          = "argocd-manager"
	GITOPSROLEKINDNAME             = "Role"
	GITOPSDATASECRETNAME           = "argocd-secret"
	GITOPSCONFIGMAPNAME            = "argocd-cm"
	GITOPSARGOCDCUSTOMRESOURCENAME = "argocd"
	GITOPSDEPLOYMENTNAME           = "argocd-server"
)

// Reconciling GitOps
func (r *WorkshopReconciler) reconcileGitOps(workshop *workshopv1.Workshop, users int,
	appsHostnameSuffix string, openshiftConsoleURL string) (reconcile.Result, error) {

	enabledGitOps := workshop.Spec.Infrastructure.GitOps.Enabled

	if enabledGitOps {
		if result, err := r.addGitOps(workshop, users, appsHostnameSuffix, openshiftConsoleURL); util.IsRequeued(result, err) {
			return result, err
		}
	}

	//Success
	return reconcile.Result{}, nil
}

// Add GitOps
func (r *WorkshopReconciler) addGitOps(workshop *workshopv1.Workshop, users int,
	appsHostnameSuffix string, openshiftConsoleURL string) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.GitOps.OperatorHub.Channel
	clusterServiceVersion := workshop.Spec.Infrastructure.GitOps.OperatorHub.ClusterServiceVersion

	labels := map[string]string{
		"app.kubernetes.io/part-of": "argocd",
	}

	// Create subscription
	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, GITOPSNAME, GITOPSOPERATORNAMESPACENAME,
		GITOPSNAME, channel, clusterServiceVersion)
	if err := r.Create(context.TODO(), subscription); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s GitOps Subscription", subscription.Name)
	}

	// Approve the installation
	if err := r.ApproveInstallPlan(clusterServiceVersion, GITOPSNAME, GITOPSOPERATORNAMESPACENAME); err != nil {
		log.Infof("Waiting for GitOps Subscription to create InstallPlan for %s", GITOPSNAME)
		return reconcile.Result{Requeue: true}, nil
	}

	// Wait for Operator to be running
	if !kubernetes.GetK8Client().GetDeploymentStatus(GITOPSDEPLOYMENTSTATUSNAME, GITOPSOPERATORNAMESPACENAME) {
		return reconcile.Result{Requeue: true}, nil
	}

	// Create a Project
	namespace := kubernetes.NewNamespace(workshop, r.Scheme, ARGOCDNAMESPACENAME)
	if err := r.Create(context.TODO(), namespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s GitOps Project", namespace.Name)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(workshop.Spec.User.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Errorf("Error when Bcrypt encrypt password for Argo CD: %v", err)
		return reconcile.Result{}, err
	}
	bcryptPassword := string(hashedPassword)

	argocdPolicy := ""
	namespaceList := ""
	secretData := map[string]string{}
	configMapData := map[string]string{}

	for id := 1; id <= users; id++ {
		username := fmt.Sprintf("user%d", id)
		userRole := fmt.Sprintf("role:%s", username)
		projectName := fmt.Sprintf("%s%d", workshop.Spec.Infrastructure.Project.StagingName, id)
		if id == 1 {
			namespaceList = projectName
		} else {
			namespaceList = fmt.Sprintf("%s,%s", namespaceList, projectName)
		}

		userPolicy := `p, ` + userRole + `, applications, *, ` + projectName + `/*, allow
p, ` + userRole + `, clusters, get, https://kubernetes.default.svc, allow
p, ` + userRole + `, projects, *,` + projectName + `, allow
p, ` + userRole + `, repositories, *, http://gitea-server.gitea.svc:3000/` + username + `/*, allow
g, ` + username + `, ` + userRole + `
`
		argocdPolicy = fmt.Sprintf("%s%s", argocdPolicy, userPolicy)

		secretData[fmt.Sprintf("accounts.%s.password", username)] = bcryptPassword

		configMapData[fmt.Sprintf("accounts.%s", username)] = "login"

		labels["app.kubernetes.io/name"] = "appproject-cr"
		appProjectCustomResource := argocd.NewAppProjectCustomResource(workshop, r.Scheme, projectName, ARGOCDNAMESPACENAME, labels, argocdPolicy)
		if err := r.Create(context.TODO(), appProjectCustomResource); err != nil && !errors.IsAlreadyExists(err) {
			return reconcile.Result{}, err
		} else if err == nil {
			log.Infof("Created %s GitOps Custom Resource", appProjectCustomResource.Name)
		} else if errors.IsAlreadyExists(err) {
			customResourceFound := &argocdv1.AppProject{}
			if err := r.Get(context.TODO(), types.NamespacedName{Name: appProjectCustomResource.Name, Namespace: ARGOCDNAMESPACENAME}, customResourceFound); err != nil {
				return reconcile.Result{}, err
			} else if err == nil {
				if !reflect.DeepEqual(appProjectCustomResource.Spec, customResourceFound.Spec) {
					customResourceFound.Spec = appProjectCustomResource.Spec
					if err := r.Update(context.TODO(), customResourceFound); err != nil {
						return reconcile.Result{}, err
					}
					log.Infof("Updated %s GitOps Custom Resource", customResourceFound.Name)
				}
			}
		}

		subjects := []rbac.Subject{}
		argocdSubject := rbac.Subject{
			Kind:     rbac.UserKind,
			Name:     "system:serviceaccount:argocd:argocd-argocd-application-controller",
			APIGroup: "rbac.authorization.k8s.io",
		}

		subjects = append(subjects, argocdSubject)

		role := kubernetes.NewRole(workshop, r.Scheme,
			GITOPSROLENAME, projectName, labels, kubernetes.ArgoCDRules())
		if err := r.Create(context.TODO(), role); err != nil && !errors.IsAlreadyExists(err) {
			return reconcile.Result{}, err
		} else if err == nil {
			log.Infof("Created %s GitOps Role in %s namespace", role.Name, projectName)
		}

		roleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme,
			GITOPSROLEBINDINGNAME, projectName, labels, subjects, role.Name, GITOPSROLEKINDNAME)
		if err := r.Create(context.TODO(), roleBinding); err != nil && !errors.IsAlreadyExists(err) {
			return reconcile.Result{}, err
		} else if err == nil {
			log.Infof("Created %s GitOps Role Binding in %s namespace", roleBinding.Name, projectName)
		} else if errors.IsAlreadyExists(err) {
			found := &rbac.RoleBinding{}
			if err := r.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: projectName}, found); err != nil {
				return reconcile.Result{}, err
			} else if err == nil {
				if !reflect.DeepEqual(subjects, found.Subjects) {
					found.Subjects = subjects
					if err := r.Update(context.TODO(), found); err != nil {
						return reconcile.Result{}, err
					}
					log.Infof("Updated %s GitOps Role Binding in %s namespace", found.Name, projectName)
				}
			}
		}
	}

	labels["app.kubernetes.io/name"] = "argocd-secret"
	secret := kubernetes.NewStringDataSecret(workshop, r.Scheme, GITOPSDATASECRETNAME, ARGOCDNAMESPACENAME, labels, secretData)
	if err := r.Create(context.TODO(), secret); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s GitOpsSecret", secret.Name)
		// } else if errors.IsAlreadyExists(err) {
		// 	secretFound := &corev1.Secret{}
		// 	if err := r.Get(context.TODO(), types.NamespacedName{Name: secret.Name, Namespace: namespace.Name}, secretFound); err != nil {
		// 		return reconcile.Result{}, err
		// 	} else if err == nil {
		// 		if !util.IsIntersectMap(secretData, secretFound.StringData) {
		// 			secretFound.StringData = secretData
		// 			if err := r.Update(context.TODO(), secretFound); err != nil {
		// 				return reconcile.Result{}, err
		// 			}
		// 			log.Infof("Updated %s Secret", secretFound.Name)
		// 		}
		// 	}
	}

	labels["app.kubernetes.io/name"] = "argocd-cm"
	configmap := kubernetes.NewConfigMap(workshop, r.Scheme, GITOPSCONFIGMAPNAME, ARGOCDNAMESPACENAME, labels, configMapData)
	if err := r.Create(context.TODO(), configmap); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s GitOps ConfigMap", configmap.Name)
	} else if errors.IsAlreadyExists(err) {
		configmapFound := &corev1.ConfigMap{}
		if err := r.Get(context.TODO(), types.NamespacedName{Name: configmap.Name, Namespace: namespace.Name}, configmapFound); err != nil {
			return reconcile.Result{}, err
		} else if err == nil {
			if !util.IsIntersectMap(configMapData, configmapFound.Data) {
				configmapFound.Data = configMapData
				if err := r.Update(context.TODO(), configmapFound); err != nil {
					return reconcile.Result{}, err
				}
				log.Infof("Updated %s GitOps ConfigMap", configmapFound.Name)
			}
		}
	}

	labels["app.kubernetes.io/name"] = "argocd-cr"
	argoCDCustomResource := argocd.NewArgoCDCustomResource(workshop, r.Scheme, GITOPSARGOCDCUSTOMRESOURCENAME, ARGOCDNAMESPACENAME, labels, argocdPolicy)
	if err := r.Create(context.TODO(), argoCDCustomResource); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s GitOps Custom Resource", argoCDCustomResource.Name)
	} else if errors.IsAlreadyExists(err) {
		customResourceFound := &argocdoperatorv1.ArgoCD{}
		if err := r.Get(context.TODO(), types.NamespacedName{Name: argoCDCustomResource.Name, Namespace: namespace.Name}, customResourceFound); err != nil {
			return reconcile.Result{}, err
		} else if err == nil {
			if !reflect.DeepEqual(&argocdPolicy, customResourceFound.Spec.RBAC.Policy) {
				customResourceFound.Spec.RBAC.Policy = &argocdPolicy
				if err := r.Update(context.TODO(), customResourceFound); err != nil {
					return reconcile.Result{}, err
				}
				log.Infof("Updated %s GitOps Custom Resource", customResourceFound.Name)
			}
		}
	}

	// Wait for ArgoCD Dex Server to be running
	// if !kubernetes.GetK8Client().GetDeploymentStatus("argocd-dex-server", namespace.Name) {
	// 	return reconcile.Result{Requeue: true}, nil
	// }

	// Wait for ArgoCD Server to be running
	if !kubernetes.GetK8Client().GetDeploymentStatus(GITOPSDEPLOYMENTNAME, namespace.Name) {
		return reconcile.Result{Requeue: true}, nil
	}

	labels["app.kubernetes.io/name"] = "argocd-default-cluster-config"

	if result, err := r.manageArgocdDefaultClusterConfigSecret(workshop, namespace.Name, labels, namespaceList); util.IsRequeued(result, err) {
		return result, err
	}

	//Success
	return reconcile.Result{}, nil
}

func (r *WorkshopReconciler) manageArgocdDefaultClusterConfigSecret(workshop *workshopv1.Workshop, namespaceName string,
	labels map[string]string, namespaceList string) (reconcile.Result, error) {

	clusterConfigSecretData["config"] = "{\"tlsClientConfig\":{\"insecure\":false}}"
	clusterConfigSecretData["name"] = "in-cluster"
	clusterConfigSecretData["namespaces"] = namespaceList
	clusterConfigSecretData["server"] = "https://kubernetes.default.svc"

	clusterConfigSecret := kubernetes.NewStringDataSecret(workshop, r.Scheme, secretName, namespaceName, labels, clusterConfigSecretData)
	if err := r.Create(context.TODO(), clusterConfigSecret); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Argocd Secret", clusterConfigSecret.Name)
	} else if errors.IsAlreadyExists(err) {
		clusterConfigSecretFound := &corev1.Secret{}
		if err := r.Get(context.TODO(), types.NamespacedName{Name: clusterConfigSecret.Name, Namespace: namespaceName}, clusterConfigSecretFound); err != nil {
			return reconcile.Result{}, err
		} else if err == nil {
			if !util.IsIntersectMap(clusterConfigSecretData, clusterConfigSecretFound.StringData) {
				clusterConfigSecretFound.StringData = clusterConfigSecretData
				if err := r.Update(context.TODO(), clusterConfigSecretFound); err != nil {
					return reconcile.Result{}, err
				}
				log.Infof("Updated %s Argocd Secret", clusterConfigSecretFound.Name)
			}
		}
	}

	//Success
	return reconcile.Result{}, nil
}

// delete GitOps
func (r *WorkshopReconciler) deleteGitOps(workshop *workshopv1.Workshop, users int,
	appsHostnameSuffix string, openshiftConsoleURL string) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.GitOps.OperatorHub.Channel
	clusterServiceVersion := workshop.Spec.Infrastructure.GitOps.OperatorHub.ClusterServiceVersion
	labels := map[string]string{
		"app.kubernetes.io/part-of": "argocd",
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(workshop.Spec.User.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Errorf("Error when Bcrypt encrypt password for Argo CD: %v", err)
		return reconcile.Result{}, err
	}
	bcryptPassword := string(hashedPassword)

	argocdPolicy := ""
	namespaceList := ""
	secretData := map[string]string{}
	configMapData := map[string]string{}

	if result, err := r.deleteArgocdDefaultClusterConfigSecret(workshop, ARGOCDNAMESPACENAME, labels, namespaceList); util.IsRequeued(result, err) {
		return result, err
	}

	labels["app.kubernetes.io/name"] = "argocd-cr"
	argoCDCustomResource := argocd.NewArgoCDCustomResource(workshop, r.Scheme, GITOPSARGOCDCUSTOMRESOURCENAME, ARGOCDNAMESPACENAME, labels, argocdPolicy)
	// Delete argoCD Custom Resource
	if err := r.Delete(context.TODO(), argoCDCustomResource); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s argoCD Custom Resource", argoCDCustomResource.Name)

	labels["app.kubernetes.io/name"] = "argocd-cm"
	configmap := kubernetes.NewConfigMap(workshop, r.Scheme, GITOPSCONFIGMAPNAME, ARGOCDNAMESPACENAME, labels, configMapData)
	// Delete Configmap
	if err := r.Delete(context.TODO(), configmap); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s argoCD Configmap", configmap.Name)

	labels["app.kubernetes.io/name"] = "argocd-secret"
	secret := kubernetes.NewStringDataSecret(workshop, r.Scheme, GITOPSDATASECRETNAME, ARGOCDNAMESPACENAME, labels, secretData)
	// Delete Secret
	if err := r.Delete(context.TODO(), secret); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s argoCD Secret", secret.Name)

	for id := 1; id <= users; id++ {
		username := fmt.Sprintf("user%d", id)
		userRole := fmt.Sprintf("role:%s", username)
		projectName := fmt.Sprintf("%s%d", workshop.Spec.Infrastructure.Project.StagingName, id)
		if id == 1 {
			namespaceList = projectName
		} else {
			namespaceList = fmt.Sprintf("%s,%s", namespaceList, projectName)
		}

		userPolicy := `p, ` + userRole + `, applications, *, ` + projectName + `/*, allow
p, ` + userRole + `, clusters, get, https://kubernetes.default.svc, allow
p, ` + userRole + `, projects, *,` + projectName + `, allow
p, ` + userRole + `, repositories, *, http://gitea-server.gitea.svc:3000/` + username + `/*, allow
g, ` + username + `, ` + userRole + `
`
		argocdPolicy = fmt.Sprintf("%s%s", argocdPolicy, userPolicy)

		secretData[fmt.Sprintf("accounts.%s.password", username)] = bcryptPassword

		configMapData[fmt.Sprintf("accounts.%s", username)] = "login"

		subjects := []rbac.Subject{}
		argocdSubject := rbac.Subject{
			Kind:     rbac.UserKind,
			Name:     "system:serviceaccount:argocd:argocd-argocd-application-controller",
			APIGroup: "rbac.authorization.k8s.io",
		}

		subjects = append(subjects, argocdSubject)

		role := kubernetes.NewRole(workshop, r.Scheme, GITOPSROLENAME, projectName, labels, kubernetes.ArgoCDRules())

		roleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme,
			GITOPSROLEBINDINGNAME, projectName, labels, subjects, role.Name, GITOPSROLEKINDNAME)
		// Delete roleBinding
		if err := r.Delete(context.TODO(), roleBinding); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s GitOps Role Binding  in %s namespace", roleBinding.Name, projectName)

		// Delete role
		if err := r.Delete(context.TODO(), role); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s GitOps role in %s namespace ", role.Name, projectName)

		labels["app.kubernetes.io/name"] = "appproject-cr"
		appProjectCustomResource := argocd.NewAppProjectCustomResource(workshop, r.Scheme, projectName, ARGOCDNAMESPACENAME, labels, argocdPolicy)
		// Delete appProject Custom Resource
		if err := r.Delete(context.TODO(), appProjectCustomResource); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s GitOps appProject Custom Resource ", appProjectCustomResource.Name)
	}

	namespace := kubernetes.NewNamespace(workshop, r.Scheme, ARGOCDNAMESPACENAME)
	// Delete a Project
	if err := r.Delete(context.TODO(), namespace); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s GitOpsProject", namespace.Name)

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, GITOPSNAME, GITOPSOPERATORNAMESPACENAME,
		GITOPSNAME, channel, clusterServiceVersion)
	// Delete subscription
	if err := r.Delete(context.TODO(), subscription); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s GitOps Subscription", subscription.Name)

	//Success
	return reconcile.Result{}, nil
}

func (r *WorkshopReconciler) deleteArgocdDefaultClusterConfigSecret(workshop *workshopv1.Workshop, namespaceName string,
	labels map[string]string, namespaceList string) (reconcile.Result, error) {

	clusterConfigSecretData["config"] = "{\"tlsClientConfig\":{\"insecure\":false}}"
	clusterConfigSecretData["name"] = "in-cluster"
	clusterConfigSecretData["namespaces"] = namespaceList
	clusterConfigSecretData["server"] = "https://kubernetes.default.svc"
	labels["app.kubernetes.io/name"] = "argocd-default-cluster-config"

	clusterConfigSecret := kubernetes.NewStringDataSecret(workshop, r.Scheme, secretName, namespaceName, labels, clusterConfigSecretData)
	// delete cluster Config Secret
	if err := r.Delete(context.TODO(), clusterConfigSecret); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Argocd Secret", clusterConfigSecret.Name)

	//Success
	return reconcile.Result{}, nil
}
