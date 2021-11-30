package controllers

import (
	"context"
	"fmt"
	external "github.com/maistra/istio-operator/pkg/apis/external/kiali/v1alpha1"
	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	maistrav2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	"github.com/stakater/workshop-operator/common/kubernetes"
	"github.com/stakater/workshop-operator/common/maistra"
	"github.com/stakater/workshop-operator/common/util"
	admissionregistration "k8s.io/api/admissionregistration/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	OPERATOR_REDHAT_NAMESPACE_NAME            = "openshift-operators-redhat"
	ELASTICSEARCH_SUBSCRIPTION_NAMESPACE_NAME = "openshift-operators-redhat"
	ELASTICSEARCH_SUBSCRIPTION_PACKAGE_NAME   = "elasticsearch-operator"
	JAEGER_SUBSCRIPTION_NAME                  = "jaeger-product"
	JAEGER_SUBSCRIPTION_NAMESPACE_NAME        = "openshift-operators"
	JAEGER_SUBSCRIPTION_PACKAGE_NAME          = "jaeger-product"
	JAEGER_ROLE_KIND_NAME                     = "Role"
	JAEGER_ROLE_NAME                          = "jaeger-user"
	JAEGER_ROLE_NAMESPACE_NAME                = "istio-system"
	JAEGER_ROLE_BINDING_NAME                  = "jaeger-users"
	JAEGER_ROLE_BINDING_NAMESPACE_NAME        = "istio-system"
	KIALI_NAME                                = "kiali"
	KIALI_SUBSCRIPTION_NAME                   = "kiali-ossm"
	KIALI_SUBSCRIPTION_NAMESPACE_NAME         = "openshift-operators"
	KIALI_SUBSCRIPTION_PACKAGE_NAME           = "kiali-ossm"
	SERVICE_MESH_SUBSCRIPTION_NAME            = "servicemeshoperator"
	SERVICE_MESH_SUBSCRIPTION_NAMESPACE_NAME  = "openshift-operators"
	SERVICE_MESH_SUBSCRIPTION_PACKAGE_NAME    = "servicemeshoperator"
	SERVICE_MESH_MEMBER_ROLL_NAME             = "default"
	SERVICE_MESH_CONTROL_PLANE_NAME           = "basic"
	SERVICE_MESH_ROLE_NAME                    = "mesh-user"
	SERVICE_MESH_ROLE_KIND_NAME               = "Role"
	SERVICE_MESH_ROLE_BINDING_NAME            = "mesh-users"
	SERVICE_MESH_ROLE_BINDING_NAMESPACE_NAME  = "istio-system"
	ISTIO_OPERATOR_NAME                       = "istio-operator"
	ISTIO_OPERATOR_NAMESPACE_NAME             = "openshift-operators"
	ISTIO_NAMESPACE_NAME                      = "istio-system"
)

var istioLabels = map[string]string{
	"app.kubernetes.io/part-of": "istio",
}

// Reconciling ServiceMesh
func (r *WorkshopReconciler) reconcileServiceMesh(workshop *workshopv1.Workshop, users int) (reconcile.Result, error) {
	enabledServiceMesh := workshop.Spec.Infrastructure.ServiceMesh.Enabled
	enabledServerless := workshop.Spec.Infrastructure.Serverless.Enabled

	if enabledServiceMesh || enabledServerless {

		if result, err := r.addElasticSearchOperator(workshop); util.IsRequeued(result, err) {
			return result, err
		}

		if result, err := r.addJaegerOperator(workshop); util.IsRequeued(result, err) {
			return result, err
		}

		if result, err := r.addKialiOperator(workshop); util.IsRequeued(result, err) {
			return result, err
		}

		if result, err := r.addServiceMesh(workshop, users); util.IsRequeued(result, err) {
			return result, err
		}
	}

	//Success
	return reconcile.Result{}, nil
}

// Add ServiceMesh
func (r *WorkshopReconciler) addServiceMesh(workshop *workshopv1.Workshop, users int) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.ServiceMesh.ServiceMeshOperatorHub.Channel
	clusterserviceversion := workshop.Spec.Infrastructure.ServiceMesh.ServiceMeshOperatorHub.ClusterServiceVersion

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, SERVICE_MESH_SUBSCRIPTION_NAME, SERVICE_MESH_SUBSCRIPTION_NAMESPACE_NAME,
		SERVICE_MESH_SUBSCRIPTION_PACKAGE_NAME, channel, clusterserviceversion)
	if err := r.Create(context.TODO(), subscription); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Subscription", subscription.Name)
	}

	if err := r.ApproveInstallPlan(clusterserviceversion, SERVICE_MESH_SUBSCRIPTION_NAME, SERVICE_MESH_SUBSCRIPTION_NAMESPACE_NAME); err != nil {
		log.Infof("Waiting for Subscription to create InstallPlan for %s", subscription.Name)
		return reconcile.Result{Requeue: true}, nil
	}

	// Wait for Operator to be running
	if !kubernetes.GetK8Client().GetDeploymentStatus(ISTIO_OPERATOR_NAME, ISTIO_OPERATOR_NAMESPACE_NAME) {
		return reconcile.Result{Requeue: true}, nil
	}

	istioSystemNamespace := kubernetes.NewNamespace(workshop, r.Scheme, ISTIO_NAMESPACE_NAME)
	if err := r.Create(context.TODO(), istioSystemNamespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Namespace", istioSystemNamespace.Name)
	}

	istioMembers := []string{}
	istioUsers := []rbac.Subject{}

	if workshop.Spec.Infrastructure.GitOps.Enabled {
		argocdSubject := rbac.Subject{
			Kind:     rbac.UserKind,
			Name:     "system:serviceaccount:argocd:argocd-argocd-application-controller",
			APIGroup: "rbac.authorization.k8s.io",
		}
		istioUsers = append(istioUsers, argocdSubject)
	}

	for id := 1; id <= users; id++ {
		username := fmt.Sprintf("user%d", id)
		stagingProjectName := fmt.Sprintf("%s%d", workshop.Spec.Infrastructure.Project.StagingName, id)
		userSubject := rbac.Subject{
			Kind:     rbac.UserKind,
			Name:     username,
			APIGroup: "rbac.authorization.k8s.io",
		}

		istioMembers = append(istioMembers, stagingProjectName)
		istioUsers = append(istioUsers, userSubject)
	}

	jaegerRole := kubernetes.NewRole(workshop, r.Scheme,
		JAEGER_ROLE_NAME, JAEGER_ROLE_NAMESPACE_NAME, istioLabels, kubernetes.JaegerUserRules())
	if err := r.Create(context.TODO(), jaegerRole); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Role", jaegerRole.Name)
	}

	jaegerRoleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme,
		JAEGER_ROLE_BINDING_NAME, JAEGER_ROLE_BINDING_NAMESPACE_NAME, istioLabels, istioUsers, jaegerRole.Name, JAEGER_ROLE_KIND_NAME)
	if err := r.Create(context.TODO(), jaegerRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Role Binding", jaegerRoleBinding.Name)
	} else if errors.IsAlreadyExists(err) {
		found := &rbac.RoleBinding{}
		if err := r.Get(context.TODO(), types.NamespacedName{Name: jaegerRoleBinding.Name, Namespace: istioSystemNamespace.Name}, found); err != nil {
			return reconcile.Result{}, err
		} else if err == nil {
			if !reflect.DeepEqual(istioUsers, found.Subjects) {
				found.Subjects = istioUsers
				if err := r.Update(context.TODO(), found); err != nil {
					return reconcile.Result{}, err
				}
				log.Infof("Updated %s Role Binding", found.Name)
			}
		}
	}

	meshUserRoleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme,
		SERVICE_MESH_ROLE_BINDING_NAME, SERVICE_MESH_ROLE_BINDING_NAMESPACE_NAME, istioLabels, istioUsers, SERVICE_MESH_ROLE_NAME, SERVICE_MESH_ROLE_KIND_NAME)

	if err := r.Create(context.TODO(), meshUserRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Role Binding", meshUserRoleBinding.Name)
	}

	serviceMeshControlPlaneCR := maistra.NewServiceMeshControlPlaneCR(workshop, r.Scheme, SERVICE_MESH_CONTROL_PLANE_NAME, istioSystemNamespace.Name)
	if err := r.Create(context.TODO(), serviceMeshControlPlaneCR); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Service Mesh Control Plane Custom Resource", serviceMeshControlPlaneCR.Name)
	}

	serviceMeshMemberRollCR := maistra.NewServiceMeshMemberRollCR(workshop, r.Scheme,
		SERVICE_MESH_MEMBER_ROLL_NAME, istioSystemNamespace.Name, istioMembers)
	if err := r.Create(context.TODO(), serviceMeshMemberRollCR); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Custom Resource", serviceMeshMemberRollCR.Name)
	} else if errors.IsAlreadyExists(err) {
		serviceMeshMemberRollCRFound := &maistrav1.ServiceMeshMemberRoll{}
		if err := r.Get(context.TODO(), types.NamespacedName{Name: serviceMeshMemberRollCR.Name, Namespace: istioSystemNamespace.Name}, serviceMeshMemberRollCRFound); err != nil {
			return reconcile.Result{}, err
		} else if err == nil {
			if !reflect.DeepEqual(istioMembers, serviceMeshMemberRollCRFound.Spec.Members) {
				serviceMeshMemberRollCRFound.Spec.Members = istioMembers
				if err := r.Update(context.TODO(), serviceMeshMemberRollCRFound); err != nil {
					return reconcile.Result{}, err
				}
				log.Infof("Updated %s Service Mesh Member Roll Custom Resource", serviceMeshMemberRollCRFound.Name)
			}
		}
	}
	//Success
	return reconcile.Result{}, nil
}

// Add ElasticSearchOperator
func (r *WorkshopReconciler) addElasticSearchOperator(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.ServiceMesh.ElasticSearchOperatorHub.Channel
	clusterserviceversion := workshop.Spec.Infrastructure.ServiceMesh.ElasticSearchOperatorHub.ClusterServiceVersion
	subcriptionName := fmt.Sprintf("elasticsearch-operator-%s", channel)

	redhatOperatorsNamespace := kubernetes.NewNamespace(workshop, r.Scheme, OPERATOR_REDHAT_NAMESPACE_NAME)
	if err := r.Create(context.TODO(), redhatOperatorsNamespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Namespace", redhatOperatorsNamespace.Name)
	}

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, subcriptionName, ELASTICSEARCH_SUBSCRIPTION_NAMESPACE_NAME,
		ELASTICSEARCH_SUBSCRIPTION_PACKAGE_NAME, channel, clusterserviceversion)
	if err := r.Create(context.TODO(), subscription); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Subscription", subscription.Name)
	}

	if err := r.ApproveInstallPlan(clusterserviceversion, subcriptionName, ELASTICSEARCH_SUBSCRIPTION_NAMESPACE_NAME); err != nil {
		log.Infof("Waiting for Subscription to create InstallPlan for %s", subscription.Name)
		return reconcile.Result{Requeue: true}, nil
	}

	//Success
	return reconcile.Result{}, nil
}

// Add JaegerOperator
func (r *WorkshopReconciler) addJaegerOperator(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.ServiceMesh.JaegerOperatorHub.Channel
	clusterserviceversion := workshop.Spec.Infrastructure.ServiceMesh.JaegerOperatorHub.ClusterServiceVersion

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, JAEGER_SUBSCRIPTION_NAME, JAEGER_SUBSCRIPTION_NAMESPACE_NAME,
		JAEGER_SUBSCRIPTION_PACKAGE_NAME, channel, clusterserviceversion)
	if err := r.Create(context.TODO(), subscription); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Subscription", subscription.Name)
	}

	if err := r.ApproveInstallPlan(clusterserviceversion, JAEGER_SUBSCRIPTION_NAME, JAEGER_SUBSCRIPTION_NAMESPACE_NAME); err != nil {
		log.Infof("Waiting for Subscription to create InstallPlan for %s", subscription.Name)
		return reconcile.Result{Requeue: true}, nil
	}

	//Success
	return reconcile.Result{}, nil
}

// Add KialiOperator
func (r *WorkshopReconciler) addKialiOperator(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.ServiceMesh.KialiOperatorHub.Channel
	clusterserviceversion := workshop.Spec.Infrastructure.ServiceMesh.KialiOperatorHub.ClusterServiceVersion

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, KIALI_SUBSCRIPTION_NAME, KIALI_SUBSCRIPTION_NAMESPACE_NAME,
		KIALI_SUBSCRIPTION_PACKAGE_NAME, channel, clusterserviceversion)
	if err := r.Create(context.TODO(), subscription); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Subscription", subscription.Name)
	}

	if err := r.ApproveInstallPlan(clusterserviceversion, KIALI_SUBSCRIPTION_NAME, KIALI_SUBSCRIPTION_NAMESPACE_NAME); err != nil {
		log.Infof("Waiting for Subscription to create InstallPlan for %s", subscription.Name)
		return reconcile.Result{Requeue: true}, nil
	}

	//Success
	return reconcile.Result{}, nil
}

func (r *WorkshopReconciler) deleteServiceMeshService(workshop *workshopv1.Workshop, userID int) (reconcile.Result, error) {

	servicemeshCSV, JaegerCSV, kialiCSV, err := r.getCSV(workshop)
	if err != nil {
		log.Error("Failed to get ClusterServiceVersion")
	}

	if result, err := r.deleteServiceMesh(workshop, userID); util.IsRequeued(result, err) {
		return result, err
	}
	if result, err := r.deleteKialiSubscription(workshop); util.IsRequeued(result, err) {
		return result, err
	}
	if result, err := r.deleteJaegerSubscription(workshop); util.IsRequeued(result, err) {
		return result, err
	}

	if result, err := r.deleteCSV(workshop, servicemeshCSV, kialiCSV, JaegerCSV); util.IsRequeued(result, err) {
		return result, err
	}

	if result, err := r.deleteIstioSystemNamespace(workshop); util.IsRequeued(result, err) {
		return result, err
	}

	if result, err := r.deleteElasticSearchOperator(workshop); util.IsRequeued(result, err) {
		return result, err
	}

	if result, err := r.PatchIstioProject(workshop); util.IsRequeued(result, err) {
		return result, err
	}

	return reconcile.Result{}, nil
}

// Delete ServiceMesh
func (r *WorkshopReconciler) deleteServiceMesh(workshop *workshopv1.Workshop, users int) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.ServiceMesh.ServiceMeshOperatorHub.Channel
	clusterserviceversion := workshop.Spec.Infrastructure.ServiceMesh.ServiceMeshOperatorHub.ClusterServiceVersion

	istioMembers := []string{}
	istioUsers := []rbac.Subject{}

	if workshop.Spec.Infrastructure.GitOps.Enabled {
		argocdSubject := rbac.Subject{
			Kind:     rbac.UserKind,
			Name:     "system:serviceaccount:argocd:argocd-argocd-application-controller",
			APIGroup: "rbac.authorization.k8s.io",
		}
		istioUsers = append(istioUsers, argocdSubject)
	}

	for id := 1; id <= users; id++ {
		username := fmt.Sprintf("user%d", id)
		stagingProjectName := fmt.Sprintf("%s%d", workshop.Spec.Infrastructure.Project.StagingName, id)
		userSubject := rbac.Subject{
			Kind:     rbac.UserKind,
			Name:     username,
			APIGroup: "rbac.authorization.k8s.io",
		}

		istioMembers = append(istioMembers, stagingProjectName)
		istioUsers = append(istioUsers, userSubject)
	}

	jaegerRole := kubernetes.NewRole(workshop, r.Scheme,
		JAEGER_ROLE_NAME, JAEGER_ROLE_NAMESPACE_NAME, istioLabels, kubernetes.JaegerUserRules())

	serviceMeshMemberRollCR := maistra.NewServiceMeshMemberRollCR(workshop, r.Scheme,
		SERVICE_MESH_MEMBER_ROLL_NAME, ISTIO_NAMESPACE_NAME, istioMembers)
	// Delete Service MeshMember Roll Custom Resource
	if err := r.Delete(context.TODO(), serviceMeshMemberRollCR); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Service MeshMember Roll Custom Resource", serviceMeshMemberRollCR.Name)

	serviceMeshControlPlaneCR := maistra.NewServiceMeshControlPlaneCR(workshop, r.Scheme, SERVICE_MESH_CONTROL_PLANE_NAME, ISTIO_NAMESPACE_NAME)
	// Delete Service Mesh Control Plane Custom Resource
	if err := r.Delete(context.TODO(), serviceMeshControlPlaneCR); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Service Mesh Control Plane Custom Resource", serviceMeshControlPlaneCR.Name)

	meshUserRoleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme,
		SERVICE_MESH_ROLE_BINDING_NAME, SERVICE_MESH_ROLE_BINDING_NAMESPACE_NAME, istioLabels, istioUsers, SERVICE_MESH_ROLE_NAME, SERVICE_MESH_ROLE_KIND_NAME)
	// Delete meshUser RoleBinding
	if err := r.Delete(context.TODO(), meshUserRoleBinding); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Role Binding", meshUserRoleBinding.Name)

	jaegerRoleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme,
		JAEGER_ROLE_BINDING_NAME, JAEGER_ROLE_BINDING_NAMESPACE_NAME, istioLabels, istioUsers, jaegerRole.Name, JAEGER_ROLE_KIND_NAME)
	// Delete jaeger RoleBinding
	if err := r.Delete(context.TODO(), jaegerRoleBinding); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Role Binding", jaegerRoleBinding.Name)

	// Delete jaeger Role
	if err := r.Delete(context.TODO(), jaegerRole); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Role", jaegerRole.Name)

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, SERVICE_MESH_SUBSCRIPTION_NAME, SERVICE_MESH_SUBSCRIPTION_NAMESPACE_NAME,
		SERVICE_MESH_SUBSCRIPTION_PACKAGE_NAME, channel, clusterserviceversion)
	// Delete Subscription
	if err := r.Delete(context.TODO(), subscription); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Subscription", subscription.Name)

	vwc := &admissionregistration.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openshift-operators.servicemesh-resources.maistra.io",
			Namespace: "openshift-operators",
		},
	}

	// Delete ValidatingWebhookConfiguration
	if err := r.Delete(context.TODO(), vwc); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("deleted %s ValidatingWebhookConfiguration", vwc.Name)

	mwc := &admissionregistration.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openshift-operators.servicemesh-resources.maistra.io",
			Namespace: "openshift-operators",
		},
	}
	// Delete MutatingWebhookConfiguration
	if err := r.Delete(context.TODO(), mwc); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("deleted %s MutatingWebhookConfiguration", mwc.Name)

	//Success
	return reconcile.Result{}, nil
}

// Delete KialiSubscription
func (r *WorkshopReconciler) deleteKialiSubscription(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.ServiceMesh.KialiOperatorHub.Channel
	clusterserviceversion := workshop.Spec.Infrastructure.ServiceMesh.KialiOperatorHub.ClusterServiceVersion

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, KIALI_SUBSCRIPTION_NAME, KIALI_SUBSCRIPTION_NAMESPACE_NAME,
		KIALI_SUBSCRIPTION_PACKAGE_NAME, channel, clusterserviceversion)
	// Delete Subscription
	if err := r.Delete(context.TODO(), subscription); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Subscription", subscription.Name)
	//Success
	return reconcile.Result{}, nil
}

// Delete JaegerSubscription
func (r *WorkshopReconciler) deleteJaegerSubscription(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.ServiceMesh.JaegerOperatorHub.Channel
	clusterserviceversion := workshop.Spec.Infrastructure.ServiceMesh.JaegerOperatorHub.ClusterServiceVersion

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, JAEGER_SUBSCRIPTION_NAME, JAEGER_SUBSCRIPTION_NAMESPACE_NAME,
		JAEGER_SUBSCRIPTION_PACKAGE_NAME, channel, clusterserviceversion)
	// Delete Subscription
	if err := r.Delete(context.TODO(), subscription); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Subscription", subscription.Name)

	//Success
	return reconcile.Result{}, nil
}

// Delete ElasticSearchOperator
func (r *WorkshopReconciler) deleteElasticSearchOperator(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.ServiceMesh.ElasticSearchOperatorHub.Channel
	clusterserviceversion := workshop.Spec.Infrastructure.ServiceMesh.ElasticSearchOperatorHub.ClusterServiceVersion
	subcriptionName := fmt.Sprintf("elasticsearch-operator-%s", channel)

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, subcriptionName, ELASTICSEARCH_SUBSCRIPTION_NAMESPACE_NAME,
		ELASTICSEARCH_SUBSCRIPTION_PACKAGE_NAME, channel, clusterserviceversion)
	// Delete Subscription
	if err := r.Delete(context.TODO(), subscription); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Subscription", subscription.Name)

	redhatOperatorsNamespace := kubernetes.NewNamespace(workshop, r.Scheme, OPERATOR_REDHAT_NAMESPACE_NAME)
	// Delete Namespace
	if err := r.Delete(context.TODO(), redhatOperatorsNamespace); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Namespace", redhatOperatorsNamespace.Name)

	//Success
	return reconcile.Result{}, nil
}

// delete IstioSystem Namespace
func (r *WorkshopReconciler) deleteIstioSystemNamespace(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	istioSystemNamespace := kubernetes.NewNamespace(workshop, r.Scheme, ISTIO_NAMESPACE_NAME)
	// Delete istio-system Namespace
	if err := r.Delete(context.TODO(), istioSystemNamespace); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Namespace", istioSystemNamespace.Name)

	//Success
	return reconcile.Result{}, nil
}

// get CSV of servicemesh, jaeger, kiali
func (r *WorkshopReconciler) getCSV(workshop *workshopv1.Workshop) (string, string, string, error) {

	servicemeshSubFound := &olmv1alpha1.Subscription{}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: SERVICE_MESH_SUBSCRIPTION_NAME, Namespace: SERVICE_MESH_SUBSCRIPTION_NAMESPACE_NAME}, servicemeshSubFound); err != nil {
		return servicemeshSubFound.Status.InstalledCSV, "", "", err
	}
	log.Info(servicemeshSubFound.Status.InstalledCSV)

	jaegerSubFound := &olmv1alpha1.Subscription{}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: JAEGER_SUBSCRIPTION_NAME, Namespace: JAEGER_SUBSCRIPTION_NAMESPACE_NAME}, jaegerSubFound); err != nil {
		return "", jaegerSubFound.Status.InstalledCSV, "", err
	}
	log.Info(jaegerSubFound.Status.InstalledCSV)

	kialiSubFound := &olmv1alpha1.Subscription{}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: KIALI_SUBSCRIPTION_NAME, Namespace: KIALI_SUBSCRIPTION_NAMESPACE_NAME}, kialiSubFound); err != nil {
		return "", "", kialiSubFound.Status.InstalledCSV, err
	}
	log.Info(kialiSubFound.Status.InstalledCSV)

	return servicemeshSubFound.Status.InstalledCSV, jaegerSubFound.Status.InstalledCSV, kialiSubFound.Status.InstalledCSV, nil
}

// delete CSV of servicemesh, jaeger, kiali
func (r *WorkshopReconciler) deleteCSV(workshop *workshopv1.Workshop, servicemeshCSV string, kialiCSV string, JaegerCSV string) (reconcile.Result, error) {

	servicemeshOperatorCSV := kubernetes.NewRedHatClusterServiceVersion(workshop, r.Scheme, servicemeshCSV, SERVICE_MESH_SUBSCRIPTION_NAMESPACE_NAME)
	if err := r.Delete(context.TODO(), servicemeshOperatorCSV); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s ClusterServiceVersion", servicemeshOperatorCSV.Name)

	kialiOperatorCSV := kubernetes.NewRedHatClusterServiceVersion(workshop, r.Scheme, kialiCSV, KIALI_SUBSCRIPTION_NAMESPACE_NAME)
	if err := r.Delete(context.TODO(), kialiOperatorCSV); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s ClusterServiceVersion", kialiOperatorCSV.Name)

	JaegerOperatorCSV := kubernetes.NewRedHatClusterServiceVersion(workshop, r.Scheme, JaegerCSV, JAEGER_SUBSCRIPTION_NAMESPACE_NAME)
	if err := r.Delete(context.TODO(), JaegerOperatorCSV); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s ClusterServiceVersion", JaegerOperatorCSV.Name)

	return reconcile.Result{}, nil
}

// Patch istio-system Project
func (r *WorkshopReconciler) PatchIstioProject(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	namespaceFound := kubernetes.NewNamespace(workshop, r.Scheme, ISTIO_NAMESPACE_NAME)

	if err := r.Get(context.TODO(), types.NamespacedName{Name: ISTIO_NAMESPACE_NAME}, namespaceFound); err != nil {
		return reconcile.Result{}, err
	}

	if namespaceFound.Spec.Finalizers[0] == "kubernetes" {
		servicemeshcontrolplanes := &maistrav2.ServiceMeshControlPlane{}
		if err := r.Get(context.TODO(), types.NamespacedName{Name: SERVICE_MESH_CONTROL_PLANE_NAME, Namespace: ISTIO_NAMESPACE_NAME}, servicemeshcontrolplanes); err != nil {
			return reconcile.Result{}, err
		}

		patch := client.MergeFrom(servicemeshcontrolplanes.DeepCopy())
		servicemeshcontrolplanes.Finalizers = nil
		if err := r.Patch(context.TODO(), servicemeshcontrolplanes, patch); err != nil {
			//log.Errorf("Failed to patch ServiceMeshControlPlane %s", servicemeshcontrolplanes.Name)
			return reconcile.Result{}, err
		}
		log.Infof("patched %s ServiceMeshControlPlane", servicemeshcontrolplanes.Name)
	}

	if namespaceFound.Spec.Finalizers[0] == "kubernetes" {
		kiali := &external.Kiali{}
		if err := r.Get(context.TODO(), types.NamespacedName{Name: KIALI_NAME, Namespace: ISTIO_NAMESPACE_NAME}, kiali); err != nil {
			return reconcile.Result{}, err
		}

		patch := client.MergeFrom(kiali.DeepCopy())
		kiali.Finalizers = nil
		if err := r.Patch(context.TODO(), kiali, patch); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("patched %s kiali", kiali.Name)
	}

	if namespaceFound.Spec.Finalizers[0] == "kubernetes" {
		serviceMeshMemberRoll := &maistrav1.ServiceMeshMemberRoll{}
		if err := r.Get(context.TODO(), types.NamespacedName{Name: SERVICE_MESH_MEMBER_ROLL_NAME, Namespace: ISTIO_NAMESPACE_NAME}, serviceMeshMemberRoll); err != nil {
			return reconcile.Result{}, err
		}

		patch := client.MergeFrom(serviceMeshMemberRoll.DeepCopy())
		serviceMeshMemberRoll.Finalizers = nil
		if err := r.Patch(context.TODO(), serviceMeshMemberRoll, patch); err != nil {
			//log.Errorf("Failed to patch ServiceMeshMemberRoll %s", serviceMeshMemberRoll.Name)
			return reconcile.Result{}, err
		}
		log.Infof("patched %s ServiceMeshMemberRoll", serviceMeshMemberRoll.Name)
	}

	//Success
	return reconcile.Result{}, nil
}
