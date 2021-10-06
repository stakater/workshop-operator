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

var (
	ExtraConfigFromValues = map[string]string{
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

	VaultServerLabels = map[string]string{
		"app":                       "vault",
		"app.kubernetes.io/name":    "vault",
		"app.kubernetes.io/part-of": "vault",
		"component":                 "server",
	}
	VaultAgentLabels = map[string]string{
		"app":                       "vault",
		"app.kubernetes.io/name":    "vault-agent-injector",
		"app.kubernetes.io/part-of": "vault",
		"component":                 "webhook",
	}
)

const (
	VAULT_NAMESPACE_NAME           = "vault"
	VAULT_STATEFULSET_NAME         = "vault"
	VAULT_SERVICE_NAME             = "vault"
	VAULT_INTERNAL_SERVICE_NAME    = "vault-internal"
	VAULT_ROLEBINDING_NAME         = "vault-server-binding"
	VAULT_ROLEBINDING_ROLE_NAME    = "system:auth-delegator"
	KIND_CLUSTER_ROLE              = "ClusterRole"
	VAULT_SERVICEACCOUNT_NAME      = "vault"
	VAULT_CONFIGMAP_NAME           = "vault-config"
	VAULTAGENT_WEBHOOK_NAME        = "vault-agent-injector-cfg"
	VAULTAGENT_DEPLOYMENT_NAME     = "vault-agent-injector"
	VAULTAGENT_SERVICE_NAME        = "vault-agent-injector"
	VAULTAGENT_ROLEBINDING_NAME    = "vault-agent-injector"
	VAULTAGENT_CLUSTERROLE_NAME    = "vault-agent-injector"
	VAULTAGENT_SERVICEACCOUNT_NAME = "vault-agent-injector"
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

	log.Infoln("Creating VaultServer ")

	// Create Namespace
	vaultNamespace := kubernetes.NewNamespace(workshop, r.Scheme, VAULT_NAMESPACE_NAME)
	if err := r.Create(context.TODO(), vaultNamespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Vault Project", vaultNamespace.Name)
	}

	configMap := kubernetes.NewConfigMap(workshop, r.Scheme, VAULT_CONFIGMAP_NAME, VAULT_NAMESPACE_NAME, VaultServerLabels, ExtraConfigFromValues)
	if err := r.Create(context.TODO(), configMap); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Vault ConfigMap", configMap.Name)
	}

	// Create Service Account
	serviceAccount := kubernetes.NewServiceAccount(workshop, r.Scheme, VAULT_SERVICEACCOUNT_NAME, VAULT_NAMESPACE_NAME, VaultServerLabels)
	if err := r.Create(context.TODO(), serviceAccount); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Vault Service Account", serviceAccount.Name)
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
			log.Infof("Updated %s SCC", privilegedSCCFound.Name)
		}
	}

	vaultSCC := vault.NewVaultSSC(workshop, r.Scheme, "privileged")
	if err := r.Create(context.TODO(), vaultSCC); err != nil && errors.IsNotFound(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Vault SCC", vaultSCC.Name)
	}

	vaultSCCFound := &securityv1.SecurityContextConstraints{}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: "privileged"}, vaultSCCFound); err != nil {
		if util.StringInSlice(serviceAccountUser, vaultSCCFound.Users) {
			log.Infoln("service Account User available ")
		}
		vaultSCC.Users = append(vaultSCC.Users, serviceAccountUser)
		if err := r.Update(context.TODO(), vaultSCC); err != nil {
			return reconcile.Result{}, err
		} else if err == nil {
			log.Infof("Updated %s SCC", vaultSCC.Name)
		}
	} else {
		if !util.StringInSlice(serviceAccountUser, vaultSCCFound.Users) {
			vaultSCC.Users = append(vaultSCC.Users, serviceAccountUser)
			if err := r.Update(context.TODO(), vaultSCC); err != nil {
				return reconcile.Result{}, err
			} else if err == nil {
				log.Infof("Updated %s SCC", vaultSCC.Name)
			}
		}
	}

	// Create ClusterRole Binding
	clusterRoleBinding := kubernetes.NewClusterRoleBindingSA(workshop, r.Scheme, VAULT_ROLEBINDING_NAME, VAULT_NAMESPACE_NAME,
		VaultServerLabels, serviceAccount.Name, VAULT_ROLEBINDING_ROLE_NAME, KIND_CLUSTER_ROLE)
	if err := r.Create(context.TODO(), clusterRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Vault Cluster Role Binding", clusterRoleBinding.Name)
	}

	// Create Service
	internalService := kubernetes.NewService(workshop, r.Scheme, VAULT_INTERNAL_SERVICE_NAME, VAULT_NAMESPACE_NAME, VaultServerLabels, []string{"http", "internal"}, []int32{8200, 8201})
	if err := r.Create(context.TODO(), internalService); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Vault Service", internalService.Name)
	}

	// Create Service
	service := kubernetes.NewService(workshop, r.Scheme, VAULT_SERVICE_NAME, VAULT_NAMESPACE_NAME, VaultServerLabels, []string{"http", "internal"}, []int32{8200, 8201})
	if err := r.Create(context.TODO(), service); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Vault Service", service.Name)
	}

	// Create StatefulSet
	stateful := vault.NewStatefulSet(workshop, r.Scheme, VAULT_STATEFULSET_NAME, VAULT_NAMESPACE_NAME, VaultServerLabels)
	if err := r.Create(context.TODO(), stateful); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Vault Stateful", stateful.Name)
	}

	//Success
	return reconcile.Result{}, nil
}

// Add VaultAgentInjector
func (r *WorkshopReconciler) addVaultAgentInjector(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	log.Infoln("Creating VaultAgent")

	// Create Namespace
	vaultNamespace := kubernetes.NewNamespace(workshop, r.Scheme, VAULT_NAMESPACE_NAME)
	if err := r.Create(context.TODO(), vaultNamespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s VaultAgent Project", vaultNamespace.Name)
	}

	// Create Service Account
	serviceAccount := kubernetes.NewServiceAccount(workshop, r.Scheme, VAULTAGENT_SERVICEACCOUNT_NAME, VAULT_NAMESPACE_NAME, VaultAgentLabels)
	if err := r.Create(context.TODO(), serviceAccount); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s VaultAgent Service Account", serviceAccount.Name)
	}
	// Add Vault ServiceAccountUser to priviliged SCC
	// TODO: Instead of adding to existing priviliged SCC; create a new one
	privilegedSCCFound := &securityv1.SecurityContextConstraints{}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: "privileged"}, privilegedSCCFound); err != nil {
		return reconcile.Result{}, err
	}

	// Create Cluster Role
	clusterRole := kubernetes.NewClusterRole(workshop, r.Scheme,
		VAULTAGENT_CLUSTERROLE_NAME, VAULT_NAMESPACE_NAME, VaultAgentLabels, kubernetes.VaultAgentInjectorRules())
	if err := r.Create(context.TODO(), clusterRole); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s VaultAgent Cluster Role", clusterRole.Name)
	}

	// Create Cluster Role Binding
	clusterRoleBinding := kubernetes.NewClusterRoleBindingSA(workshop, r.Scheme, VAULTAGENT_ROLEBINDING_NAME, VAULT_NAMESPACE_NAME,
		VaultAgentLabels, VAULTAGENT_SERVICEACCOUNT_NAME, clusterRole.Name, KIND_CLUSTER_ROLE)
	if err := r.Create(context.TODO(), clusterRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s VaultAgent Cluster Role Binding", clusterRoleBinding.Name)
	}

	// Create Service
	service := kubernetes.NewServiceWithTarget(workshop, r.Scheme, VAULTAGENT_SERVICE_NAME, VAULT_NAMESPACE_NAME, VaultAgentLabels,
		[]string{"http"}, []int32{443}, []int32{8080})
	if err := r.Create(context.TODO(), service); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s VaultAgent Service", service.Name)
	}

	// Create Deployment
	ocpDeployment := vault.NewAgentInjectorDeployment(workshop, r.Scheme, VAULTAGENT_DEPLOYMENT_NAME, VAULT_NAMESPACE_NAME, VaultAgentLabels)
	if err := r.Create(context.TODO(), ocpDeployment); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s VaultAgent Deployment", ocpDeployment.Name)
	}

	// Create AgentInjectorWebHook
	webhooks := vault.NewAgentInjectorWebHook(vaultNamespace.Name)
	mutatingWebhookConfiguration := kubernetes.NewMutatingWebhookConfiguration(workshop, r.Scheme,
		VAULTAGENT_WEBHOOK_NAME, VaultAgentLabels, webhooks)
	if err := r.Create(context.TODO(), mutatingWebhookConfiguration); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s VaultAgent Mutating Webhook Configuration", mutatingWebhookConfiguration.Name)
	}

	//Success
	return reconcile.Result{}, nil
}

func (r *WorkshopReconciler) deleteVault(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	log.Infoln("Deleting Vault ")

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
	log.Infoln("Deleting VaultServer")

	// last method last line
	serviceAccount := kubernetes.NewServiceAccount(workshop, r.Scheme, VAULT_SERVICEACCOUNT_NAME, VAULT_NAMESPACE_NAME, VaultServerLabels)

	stateful := vault.NewStatefulSet(workshop, r.Scheme, VAULT_STATEFULSET_NAME, VAULT_NAMESPACE_NAME, VaultServerLabels)
	// Delete stateful
	if err := r.Delete(context.TODO(), stateful); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s VaultServer stateful", stateful.Name)

	service := kubernetes.NewService(workshop, r.Scheme, VAULT_SERVICE_NAME, VAULT_NAMESPACE_NAME, VaultServerLabels, []string{"http", "internal"}, []int32{8200, 8201})
	// Delete Service
	if err := r.Delete(context.TODO(), service); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s VaultServer Service", service.Name)

	internalService := kubernetes.NewService(workshop, r.Scheme, VAULT_INTERNAL_SERVICE_NAME, VAULT_NAMESPACE_NAME, VaultServerLabels, []string{"http", "internal"}, []int32{8200, 8201})
	// Delete internal Service
	if err := r.Delete(context.TODO(), internalService); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s VaultServer internal Service", internalService.Name)

	clusterRoleBinding := kubernetes.NewClusterRoleBindingSA(workshop, r.Scheme, VAULT_ROLEBINDING_NAME, VAULT_NAMESPACE_NAME,
		VaultServerLabels, serviceAccount.Name, VAULT_ROLEBINDING_ROLE_NAME, KIND_CLUSTER_ROLE)
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

	configMap := kubernetes.NewConfigMap(workshop, r.Scheme, VAULT_CONFIGMAP_NAME, VAULT_NAMESPACE_NAME, VaultServerLabels, ExtraConfigFromValues)
	// Delete configMap
	if err := r.Delete(context.TODO(), configMap); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s VaultServer configMap", configMap.Name)
	log.Infoln("Deleted VaultServer Successfully")

	//Success
	return reconcile.Result{}, nil
}

// delete VaultAgentInjector
func (r *WorkshopReconciler) deleteVaultAgentInjector(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	log.Infoln("Deleting VaultAgent ")

	vaultNamespace := kubernetes.NewNamespace(workshop, r.Scheme, VAULT_NAMESPACE_NAME)
	clusterRole := kubernetes.NewClusterRole(workshop, r.Scheme,
		VAULTAGENT_CLUSTERROLE_NAME, VAULT_NAMESPACE_NAME, VaultAgentLabels, kubernetes.VaultAgentInjectorRules())

	webhooks := vault.NewAgentInjectorWebHook(vaultNamespace.Name)
	mutatingWebhookConfiguration := kubernetes.NewMutatingWebhookConfiguration(workshop, r.Scheme,
		VAULTAGENT_WEBHOOK_NAME, VaultAgentLabels, webhooks)
	// Delete AgentInjectorWebHook
	if err := r.Delete(context.TODO(), mutatingWebhookConfiguration); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s VaultAgent Mutating Webhook Configuration ", mutatingWebhookConfiguration.Name)

	ocpDeployment := vault.NewAgentInjectorDeployment(workshop, r.Scheme, VAULTAGENT_DEPLOYMENT_NAME, VAULT_NAMESPACE_NAME, VaultAgentLabels)
	// Delete Deployment
	if err := r.Delete(context.TODO(), ocpDeployment); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s VaultAgent Deployment ", ocpDeployment.Name)

	service := kubernetes.NewServiceWithTarget(workshop, r.Scheme, VAULTAGENT_SERVICE_NAME, VAULT_NAMESPACE_NAME, VaultAgentLabels,
		[]string{"http"}, []int32{443}, []int32{8080})
	// Delete Service
	if err := r.Delete(context.TODO(), service); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s VaultAgent Service ", service.Name)

	clusterRoleBinding := kubernetes.NewClusterRoleBindingSA(workshop, r.Scheme, VAULTAGENT_ROLEBINDING_NAME, VAULT_NAMESPACE_NAME,
		VaultAgentLabels, VAULTAGENT_SERVICEACCOUNT_NAME, clusterRole.Name, KIND_CLUSTER_ROLE)
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

	serviceAccount := kubernetes.NewServiceAccount(workshop, r.Scheme, VAULTAGENT_SERVICEACCOUNT_NAME, VAULT_NAMESPACE_NAME, VaultAgentLabels)
	// Delete  Service Account
	if err := r.Delete(context.TODO(), serviceAccount); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s  VaultAgent Service Account", serviceAccount.Name)
	log.Infoln("Deleted VaultAgent Successfully")
	//Success
	return reconcile.Result{}, nil
}

// delete Vault Namespace
func (r *WorkshopReconciler) deleteVaultNamespace(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	log.Infoln("Deleting Namespace")
	vaultNamespace := kubernetes.NewNamespace(workshop, r.Scheme, VAULT_NAMESPACE_NAME)
	// Delete Namespace
	if err := r.Delete(context.TODO(), vaultNamespace); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Namespace", vaultNamespace.Name)
	log.Infoln("Deleted Namespace Successfully")
	//Success
	return reconcile.Result{}, nil
}
