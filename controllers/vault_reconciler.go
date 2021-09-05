package controllers

import (
	"context"

	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	"github.com/stakater/workshop-operator/common/kubernetes"
	"github.com/stakater/workshop-operator/common/util"
	"github.com/stakater/workshop-operator/common/vault"

	securityv1 "github.com/openshift/api/security/v1"
	"github.com/prometheus/common/log"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconciling Vault
func (r *WorkshopReconciler) reconcileVault(workshop *workshopv1.Workshop, users int) (reconcile.Result, error) {

	enabled := workshop.Spec.Infrastructure.Vault.Enabled
	vaultNamespaceName := "vault"

	if enabled {
		if result, err := r.addVaultServer(workshop, users, vaultNamespaceName); util.IsRequeued(result, err) {
			return result, err
		}

		if result, err := r.addVaultAgentInjector(workshop, users, vaultNamespaceName); util.IsRequeued(result, err) {
			return result, err
		}
	}

	//Success
	return reconcile.Result{}, nil
}

// Add Vault Server
func (r *WorkshopReconciler) addVaultServer(workshop *workshopv1.Workshop, users int, vaultNamespaceName string) (reconcile.Result, error) {

	// Create Labels
	labels := map[string]string{
		"app":                       "vault",
		"app.kubernetes.io/name":    "vault",
		"app.kubernetes.io/part-of": "vault",
		"component":                 "server",
	}

	// Create Namespace
	vaultNamespace := kubernetes.NewNamespace(workshop, r.Scheme, vaultNamespaceName)
	if err := r.Create(context.TODO(), vaultNamespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Project", vaultNamespace.Name)
	}

	// Create ConfigMap
	extraconfigFromValues := map[string]string{
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

	configMap := kubernetes.NewConfigMap(workshop, r.Scheme, "vault-config", vaultNamespace.Name, labels, extraconfigFromValues)
	if err := r.Create(context.TODO(), configMap); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s ConfigMap", configMap.Name)
	}

	// Create Service Account
	serviceAccount := kubernetes.NewServiceAccount(workshop, r.Scheme, "vault", vaultNamespace.Name, labels)
	if err := r.Create(context.TODO(), serviceAccount); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Service Account", serviceAccount.Name)
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

	// Create ClusterRole Binding
	clusterRoleBinding := kubernetes.NewClusterRoleBindingSA(workshop, r.Scheme, "vault-server-binding", vaultNamespace.Name,
		labels, serviceAccount.Name, "system:auth-delegator", "ClusterRole")
	if err := r.Create(context.TODO(), clusterRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Cluster Role Binding", clusterRoleBinding.Name)
	}

	// Create Service
	internalService := kubernetes.NewService(workshop, r.Scheme, "vault-internal", vaultNamespace.Name, labels, []string{"http", "internal"}, []int32{8200, 8201})
	if err := r.Create(context.TODO(), internalService); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Service", internalService.Name)
	}

	// Create Service
	service := kubernetes.NewService(workshop, r.Scheme, "vault", vaultNamespace.Name, labels, []string{"http", "internal"}, []int32{8200, 8201})
	if err := r.Create(context.TODO(), service); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Service", service.Name)
	}

	// Create StatefulSet
	stateful := vault.NewStatefulSet(workshop, r.Scheme, "vault", vaultNamespace.Name, labels)
	if err := r.Create(context.TODO(), stateful); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Stateful", stateful.Name)
	}

	//Success
	return reconcile.Result{}, nil
}

// Add VaultAgentInjector
func (r *WorkshopReconciler) addVaultAgentInjector(workshop *workshopv1.Workshop, users int, vaultNamespaceName string) (reconcile.Result, error) {

	// Create Labels
	labels := map[string]string{
		"app":                       "vault",
		"app.kubernetes.io/name":    "vault-agent-injector",
		"app.kubernetes.io/part-of": "vault",
		"component":                 "webhook",
	}

	// Create Namespace
	vaultNamespace := kubernetes.NewNamespace(workshop, r.Scheme, vaultNamespaceName)
	if err := r.Create(context.TODO(), vaultNamespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Project", vaultNamespace.Name)
	}

	// Create Service Account
	serviceAccount := kubernetes.NewServiceAccount(workshop, r.Scheme, "vault-agent-injector", vaultNamespace.Name, labels)
	if err := r.Create(context.TODO(), serviceAccount); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Service Account", serviceAccount.Name)
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
			log.Infof("Updated %s SCC", privilegedSCCFound.Name)
		}
	}

	// Create Cluster Role
	clusterRole := kubernetes.NewClusterRole(workshop, r.Scheme,
		"vault-agent-injector", vaultNamespace.Name, labels, kubernetes.VaultAgentInjectorRules())
	if err := r.Create(context.TODO(), clusterRole); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Cluster Role", clusterRole.Name)
	}

	// Create Cluster Role Binding
	clusterRoleBinding := kubernetes.NewClusterRoleBindingSA(workshop, r.Scheme, "vault-agent-injector", vaultNamespace.Name,
		labels, "vault-agent-injector", clusterRole.Name, "ClusterRole")
	if err := r.Create(context.TODO(), clusterRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Cluster Role Binding", clusterRoleBinding.Name)
	}

	// Create Service
	service := kubernetes.NewServiceWithTarget(workshop, r.Scheme, "vault-agent-injector", vaultNamespace.Name, labels,
		[]string{"http"}, []int32{443}, []int32{8080})
	if err := r.Create(context.TODO(), service); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Service", service.Name)
	}

	// Create Deployment
	ocpDeployment := vault.NewAgentInjectorDeployment(workshop, r.Scheme, "vault-agent-injector", vaultNamespace.Name, labels)
	if err := r.Create(context.TODO(), ocpDeployment); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Deployment", ocpDeployment.Name)
	}

	// Create AgentInjectorWebHook
	webhooks := vault.NewAgentInjectorWebHook(vaultNamespace.Name)
	mutatingWebhookConfiguration := kubernetes.NewMutatingWebhookConfiguration(workshop, r.Scheme,
		"vault-agent-injector-cfg", labels, webhooks)
	if err := r.Create(context.TODO(), mutatingWebhookConfiguration); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Mutating Webhook Configuration", mutatingWebhookConfiguration.Name)
	}

	//Success
	return reconcile.Result{}, nil
}

// TODO: delete Vault
// TODO: delete VaultAgentInjector
// TODO: delete namespace
