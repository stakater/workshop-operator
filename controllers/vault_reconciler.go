package controllers

import (
	"context"
	securityv1 "github.com/openshift/api/security/v1"
	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	"github.com/stakater/workshop-operator/common/kubernetes"
	"github.com/stakater/workshop-operator/common/util"
	"github.com/stakater/workshop-operator/common/vault"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var extraconfigFromValues = map[string]string{
	"extraconfig-from-values.hcl": `disable_mlock = true
ui = true

listener "tcp" {
	tls_disable = 1
	address = "[::]:8200"
	cluster_address = "[::]:8201"
}
storage "file" {
	path = "/vault/data"
}
`,
}

var VaultServerlabels = map[string]string{
	"app":                       "vault",
	"app.kubernetes.io/name":    "vault",
	"app.kubernetes.io/part-of": "vault",
	"component":                 "server",
}

var VaultAgentlabels = map[string]string{
	"app":                       "vault",
	"app.kubernetes.io/name":    "vault-agent-injector",
	"app.kubernetes.io/part-of": "vault",
	"component":                 "webhook",
}

const (
	VAULTNAMESPACENAME            = "vault"
	VAULTSTATEFULSETNAME          = "vault"
	VAULTSERVICENAME              = "vault"
	VAULTINTERNALSERVICENAME      = "vault-internal"
	VAULTROLEBINDINGNAME          = "vault-server-binding"
	VAULTROLEBINDINGROLENAME      = "system:auth-delegator"
	VAULTROLEBINDINGKINDNAME      = "ClusterRole"
	VAULTSERVICEACCOUNTNAME       = "vault"
	VAULTCONFIGMAPNAME            = "vault-config"
	VAULTAGENTWEBHOOKNAME         = "vault-agent-injector-cfg"
	VAULTAGENTDEPLOYMENTNAME      = "vault-agent-injector"
	VAULTAGENTSERVICENAME         = "vault-agent-injector"
	VAULTAGENTROLEBINDINGNAME     = "vault-agent-injector"
	VAULTAGENTROLEBINDINGKINDNAME = "ClusterRole"
	VAULTAGENTCLUSTERROLENAME     = "vault-agent-injector"
	VAULTAGENTSERVICEACCOUNTNAME  = "vault-agent-injector"
)

// Reconciling Vault
func (r *WorkshopReconciler) reconcileVault(workshop *workshopv1.Workshop, users int) (reconcile.Result, error) {

	enabled := workshop.Spec.Infrastructure.Vault.Enabled

	if enabled {
		if result, err := r.addVaultServer(workshop); util.IsRequeued(result, err) {
			return result, err
		}

		if result, err := r.addVaultAgentInjector(workshop); util.IsRequeued(result, err) {
			return result, err
		}
	}

	//Success
	return reconcile.Result{}, nil
}

// Add Vault Server
func (r *WorkshopReconciler) addVaultServer(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	log.Infoln("Creating vault Project")

	// Create Namespace
	vaultNamespace := kubernetes.NewNamespace(workshop, r.Scheme, VAULTNAMESPACENAME)
	if err := r.Create(context.TODO(), vaultNamespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s vault Project", vaultNamespace.Name)
	}

	configMap := kubernetes.NewConfigMap(workshop, r.Scheme, VAULTCONFIGMAPNAME, VAULTNAMESPACENAME, VaultServerlabels, extraconfigFromValues)
	if err := r.Create(context.TODO(), configMap); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s vault ConfigMap", configMap.Name)
	}

	// Create Service Account
	serviceAccount := kubernetes.NewServiceAccount(workshop, r.Scheme, VAULTSERVICEACCOUNTNAME, VAULTNAMESPACENAME, VaultServerlabels)
	if err := r.Create(context.TODO(), serviceAccount); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s vault Service Account", serviceAccount.Name)
	}

	// Create ServiceAccountUser
	serviceAccountUser := "system:serviceaccount:" + vaultNamespace.Name + ":" + serviceAccount.Name

	// Add Vault ServiceAccountUser to priviliged SCC
	// TODO: Create new previliged SCC for vault and use it
	privilegedSCCFound := &securityv1.SecurityContextConstraints{}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: "privileged"}, privilegedSCCFound); err != nil {
		return reconcile.Result{}, err
	}
	if !util.StringInSlice(serviceAccountUser, privilegedSCCFound.Users) {
		privilegedSCCFound.Users = append(privilegedSCCFound.Users, serviceAccountUser)
		if err := r.Update(context.TODO(), privilegedSCCFound); err != nil {
			return reconcile.Result{}, err
		} else if err == nil {
			log.Infof("Updated %s vault SCC", privilegedSCCFound.Name)
		}
	}

	// Create ClusterRole Binding
	clusterRoleBinding := kubernetes.NewClusterRoleBindingSA(workshop, r.Scheme, VAULTROLEBINDINGNAME, VAULTNAMESPACENAME,
		VaultServerlabels, serviceAccount.Name, VAULTROLEBINDINGROLENAME, VAULTROLEBINDINGKINDNAME)
	if err := r.Create(context.TODO(), clusterRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s vault Cluster Role Binding", clusterRoleBinding.Name)
	}

	// Create Service
	internalService := kubernetes.NewService(workshop, r.Scheme, VAULTINTERNALSERVICENAME, VAULTNAMESPACENAME, VaultServerlabels, []string{"http", "internal"}, []int32{8200, 8201})
	if err := r.Create(context.TODO(), internalService); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s vault Service", internalService.Name)
	}

	// Create Service
	service := kubernetes.NewService(workshop, r.Scheme, VAULTSERVICENAME, VAULTNAMESPACENAME, VaultServerlabels, []string{"http", "internal"}, []int32{8200, 8201})
	if err := r.Create(context.TODO(), service); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s vault Service", service.Name)
	}

	// Create StatefulSet
	stateful := vault.NewStatefulSet(workshop, r.Scheme, VAULTSTATEFULSETNAME, VAULTNAMESPACENAME, VaultServerlabels)
	if err := r.Create(context.TODO(), stateful); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s vault Stateful", stateful.Name)
	}

	//Success
	return reconcile.Result{}, nil
}

// Add VaultAgentInjector
func (r *WorkshopReconciler) addVaultAgentInjector(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	log.Infoln("Creating VaultAgent Project")

	// Create Namespace
	vaultNamespace := kubernetes.NewNamespace(workshop, r.Scheme, VAULTNAMESPACENAME)
	if err := r.Create(context.TODO(), vaultNamespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s VaultAgent Project", vaultNamespace.Name)
	}

	// Create Service Account
	serviceAccount := kubernetes.NewServiceAccount(workshop, r.Scheme, VAULTAGENTSERVICEACCOUNTNAME, VAULTNAMESPACENAME, VaultAgentlabels)
	if err := r.Create(context.TODO(), serviceAccount); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s VaultAgent Service Account", serviceAccount.Name)
	}

	// Create ServiceAccountUser
	serviceAccountUser := "system:serviceaccount:" + vaultNamespace.Name + ":" + serviceAccount.Name

	// Add Vault ServiceAccountUser to priviliged SCC
	// TODO: Instead of adding to existing priviliged SCC; create a new one
	privilegedSCCFound := &securityv1.SecurityContextConstraints{}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: "privileged"}, privilegedSCCFound); err != nil {
		return reconcile.Result{}, err
	}
	if !util.StringInSlice(serviceAccountUser, privilegedSCCFound.Users) {
		privilegedSCCFound.Users = append(privilegedSCCFound.Users, serviceAccountUser)
		if err := r.Update(context.TODO(), privilegedSCCFound); err != nil {
			return reconcile.Result{}, err
		} else if err == nil {
			log.Infof("Updated %s VaultAgent SCC", privilegedSCCFound.Name)
		}
	}

	// Create Cluster Role
	clusterRole := kubernetes.NewClusterRole(workshop, r.Scheme,
		VAULTAGENTCLUSTERROLENAME, VAULTNAMESPACENAME, VaultAgentlabels, kubernetes.VaultAgentInjectorRules())
	if err := r.Create(context.TODO(), clusterRole); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s VaultAgent Cluster Role", clusterRole.Name)
	}

	// Create Cluster Role Binding
	clusterRoleBinding := kubernetes.NewClusterRoleBindingSA(workshop, r.Scheme, VAULTAGENTROLEBINDINGNAME, VAULTNAMESPACENAME,
		VaultAgentlabels, VAULTAGENTSERVICEACCOUNTNAME, clusterRole.Name, VAULTAGENTROLEBINDINGKINDNAME)
	if err := r.Create(context.TODO(), clusterRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s VaultAgent Cluster Role Binding", clusterRoleBinding.Name)
	}

	// Create Service
	service := kubernetes.NewServiceWithTarget(workshop, r.Scheme, VAULTAGENTSERVICENAME, VAULTNAMESPACENAME, VaultAgentlabels,
		[]string{"http"}, []int32{443}, []int32{8080})
	if err := r.Create(context.TODO(), service); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s VaultAgent Service", service.Name)
	}

	// Create Deployment
	ocpDeployment := vault.NewAgentInjectorDeployment(workshop, r.Scheme, VAULTAGENTDEPLOYMENTNAME, VAULTNAMESPACENAME, VaultAgentlabels)
	if err := r.Create(context.TODO(), ocpDeployment); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s VaultAgent Deployment", ocpDeployment.Name)
	}

	// Create AgentInjectorWebHook
	webhooks := vault.NewAgentInjectorWebHook(vaultNamespace.Name)
	mutatingWebhookConfiguration := kubernetes.NewMutatingWebhookConfiguration(workshop, r.Scheme,
		VAULTAGENTWEBHOOKNAME, VaultAgentlabels, webhooks)
	if err := r.Create(context.TODO(), mutatingWebhookConfiguration); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s VaultAgent Mutating Webhook Configuration", mutatingWebhookConfiguration.Name)
	}

	//Success
	return reconcile.Result{}, nil
}



func (r *WorkshopReconciler) deleteVault(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	log.Infoln("Deleting deleteVault ")

	if result, err := r.deleteVaultServer(workshop); util.IsRequeued(result, err) {
		return result, err
	}
	if result, err := r.deleteVaultAgentInjector(workshop); util.IsRequeued(result, err) {
		return result, err
	}

	if result, err := r.deleteVaultNamespace(workshop); util.IsRequeued(result, err) {
		return result, err
	}
	return reconcile.Result{}, nil
}

// delete Vault
func (r *WorkshopReconciler) deleteVaultServer(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	log.Infoln("Deleting VaultServer Project")

	// last method last line
	serviceAccount := kubernetes.NewServiceAccount(workshop, r.Scheme, VAULTSERVICEACCOUNTNAME, VAULTNAMESPACENAME, VaultServerlabels)

	stateful := vault.NewStatefulSet(workshop, r.Scheme, VAULTSTATEFULSETNAME, VAULTNAMESPACENAME, VaultServerlabels)
	// Delete stateful
	if err := r.Delete(context.TODO(), stateful); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s VaultServer stateful", stateful.Name)

	service := kubernetes.NewService(workshop, r.Scheme, VAULTSERVICENAME, VAULTNAMESPACENAME, VaultServerlabels, []string{"http", "internal"}, []int32{8200, 8201})
	// Delete Service
	if err := r.Delete(context.TODO(), service); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s VaultServer Service", service.Name)

	internalService := kubernetes.NewService(workshop, r.Scheme, VAULTINTERNALSERVICENAME, VAULTNAMESPACENAME, VaultServerlabels, []string{"http", "internal"}, []int32{8200, 8201})
	// Delete internal Service
	if err := r.Delete(context.TODO(), internalService); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s VaultServer internal Service", internalService.Name)

	clusterRoleBinding := kubernetes.NewClusterRoleBindingSA(workshop, r.Scheme, VAULTROLEBINDINGNAME, VAULTNAMESPACENAME,
		VaultServerlabels, serviceAccount.Name, VAULTROLEBINDINGROLENAME, VAULTROLEBINDINGKINDNAME)
	// Delete ClusterRole Binding
	if err := r.Delete(context.TODO(), clusterRoleBinding); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s  VaultServer ClusterRole Binding", clusterRoleBinding.Name)

	// Delete Service Account
	if err := r.Delete(context.TODO(), serviceAccount); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s VaultServer Service Account", serviceAccount.Name)

	configMap := kubernetes.NewConfigMap(workshop, r.Scheme, VAULTCONFIGMAPNAME, VAULTNAMESPACENAME, VaultServerlabels, extraconfigFromValues)
	// Delete configMap
	if err := r.Delete(context.TODO(), configMap); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s VaultServer configMap", configMap.Name)
	log.Infoln("Deleted VaultServer Success")

	//Success
	return reconcile.Result{}, nil
}

// delete VaultAgentInjector
func (r *WorkshopReconciler) deleteVaultAgentInjector(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	log.Infoln("Deleting VaultAgent Project")

	vaultNamespace := kubernetes.NewNamespace(workshop, r.Scheme, VAULTNAMESPACENAME)
	clusterRole := kubernetes.NewClusterRole(workshop, r.Scheme,
		VAULTAGENTCLUSTERROLENAME, VAULTNAMESPACENAME, VaultAgentlabels, kubernetes.VaultAgentInjectorRules())

	webhooks := vault.NewAgentInjectorWebHook(vaultNamespace.Name)
	mutatingWebhookConfiguration := kubernetes.NewMutatingWebhookConfiguration(workshop, r.Scheme,
		VAULTAGENTWEBHOOKNAME, VaultAgentlabels, webhooks)
	// Delete AgentInjectorWebHook
	if err := r.Delete(context.TODO(), mutatingWebhookConfiguration); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s VaultAgent Mutating Webhook Configuration ", mutatingWebhookConfiguration.Name)

	ocpDeployment := vault.NewAgentInjectorDeployment(workshop, r.Scheme, VAULTAGENTDEPLOYMENTNAME, VAULTNAMESPACENAME, VaultAgentlabels)
	// Delete Deployment
	if err := r.Delete(context.TODO(), ocpDeployment); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s VaultAgent Deployment ", ocpDeployment.Name)

	service := kubernetes.NewServiceWithTarget(workshop, r.Scheme, VAULTAGENTSERVICENAME, VAULTNAMESPACENAME, VaultAgentlabels,
		[]string{"http"}, []int32{443}, []int32{8080})
	// Delete Service
	if err := r.Delete(context.TODO(), service); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s VaultAgent Service ", service.Name)

	clusterRoleBinding := kubernetes.NewClusterRoleBindingSA(workshop, r.Scheme, VAULTAGENTROLEBINDINGNAME, VAULTNAMESPACENAME,
		VaultAgentlabels, VAULTAGENTSERVICEACCOUNTNAME, clusterRole.Name, VAULTAGENTROLEBINDINGKINDNAME)
	// Delete Cluster Role Binding
	if err := r.Delete(context.TODO(), clusterRoleBinding); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s VaultAgent Cluster Role Binding", clusterRoleBinding.Name)

	// Delete Cluster Role
	if err := r.Delete(context.TODO(), clusterRole); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s VaultAgent Cluster Role", clusterRole.Name)

	serviceAccount := kubernetes.NewServiceAccount(workshop, r.Scheme, VAULTAGENTSERVICEACCOUNTNAME, VAULTNAMESPACENAME, VaultAgentlabels)
	// Delete  Service Account
	if err := r.Delete(context.TODO(), serviceAccount); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s  VaultAgent Service Account", serviceAccount.Name)
	log.Infoln("Deleted VaultAgent Success")
	//Success
	return reconcile.Result{}, nil
}

// delete Vault Namespace
func (r *WorkshopReconciler) deleteVaultNamespace(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	log.Infoln("Deleting Namespace")
	vaultNamespace := kubernetes.NewNamespace(workshop, r.Scheme, VAULTNAMESPACENAME)
	// Delete Namespace
	if err := r.Delete(context.TODO(), vaultNamespace); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Namespace", vaultNamespace.Name)
	log.Infoln("Deleted Namespace Success")
	//Success
	return reconcile.Result{}, nil
}
