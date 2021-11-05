package controllers

import (
	"context"
	"fmt"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"reflect"

	maistrav1 "github.com/maistra/istio-operator/pkg/apis/maistra/v1"
	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	"github.com/stakater/workshop-operator/common/kubernetes"
	"github.com/stakater/workshop-operator/common/maistra"
	"github.com/stakater/workshop-operator/common/util"

	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	REDHAT_OPERATOR_NAMESPACE_NAME            = "openshift-operators-redhat"
	ELASTICSEARCH_SUBSCRIPTION_NAMESPACE_NAME = "openshift-operators-redhat"
	ELASTICSEARCH_PACKAGE_NAME                = "elasticsearch-operator"
	JAEGER_SUBSCRIPTION_NAMESPACE_NAME        = "openshift-operators"
	JAEGER_SUBSCRIPTION_NAME                  = "jaeger-product"
	JAEGER_PACKAGE_NAME                       = "jaeger-product"
	JAEGER_ROLE_NAME                          = "jaeger-user"
	JAEGER_ROLE_NAMESPACE_NAME                = "istio-system"
	JAEGER_ROLE_BINDING_NAME                  = "jaeger-users"
	JAEGER_ROLE_BINDING_NAMESPACE_NAME        = "istio-system"
	JAEGER_ROLE_KIND_NAME                     = "Role"
	KIALI_SUBSCRIPTION_NAMESPACE_NAME         = "openshift-operators"
	KIALI_SUBSCRIPTION_NAME                   = "kiali-ossm"
	KIALI_PACKAGE_NAME                        = "kiali-ossm"
	SERVICE_MESH_SUBSCRIPTION_NAMESPACE_NAME  = "openshift-operators"
	SERVICE_MESH_PACKAGE_NAME                 = "servicemeshoperator"
	SERVICE_MESH_SUBSCRIPTION_NAME            = "servicemeshoperator"
	ISTIO_NAMESPACE_NAME                      = "istio-system"
	SERVICE_MESH_MEMBER_ROLL_CR_NAME          = "default"
	SERVICE_MESH_CONTROL_PLANE_CR_NAME        = "basic"
	SERVICE_MESH_DEPLOYMENT_NAME              = "istio-operator"
	SERVICE_MESH_ROLE_BINDING_NAME            = "mesh-users"
	SERVICE_MESH_ROLE_BINDING_NAMESPACE_NAME  = "istio-system"
	SERVICE_MESH_ROLE_NAME                    = "mesh-user"
	SERVICE_MESH_ROLE_KIND_NAME               = "Role"
)

var IstioLabels = map[string]string{
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

	// Service Mesh Operator
	channel := workshop.Spec.Infrastructure.ServiceMesh.ServiceMeshOperatorHub.Channel
	clusterserviceversion := workshop.Spec.Infrastructure.ServiceMesh.ServiceMeshOperatorHub.ClusterServiceVersion

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, SERVICE_MESH_SUBSCRIPTION_NAME, SERVICE_MESH_SUBSCRIPTION_NAMESPACE_NAME,
		SERVICE_MESH_PACKAGE_NAME, channel, clusterserviceversion)
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
	if !kubernetes.GetK8Client().GetDeploymentStatus(SERVICE_MESH_DEPLOYMENT_NAME, SERVICE_MESH_SUBSCRIPTION_NAMESPACE_NAME) {
		return reconcile.Result{Requeue: true}, nil
	}

	// Deploy Service Mesh
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
		JAEGER_ROLE_NAME, JAEGER_ROLE_NAMESPACE_NAME, IstioLabels, kubernetes.JaegerUserRules())
	if err := r.Create(context.TODO(), jaegerRole); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Role", jaegerRole.Name)
	}

	jaegerRoleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme,
		JAEGER_ROLE_BINDING_NAME, JAEGER_ROLE_BINDING_NAMESPACE_NAME, IstioLabels, istioUsers, jaegerRole.Name, JAEGER_ROLE_KIND_NAME)
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
		SERVICE_MESH_ROLE_BINDING_NAME, SERVICE_MESH_ROLE_BINDING_NAMESPACE_NAME, IstioLabels, istioUsers, SERVICE_MESH_ROLE_NAME, SERVICE_MESH_ROLE_KIND_NAME)

	if err := r.Create(context.TODO(), meshUserRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Role Binding", meshUserRoleBinding.Name)
	}

	serviceMeshControlPlaneCR := maistra.NewServiceMeshControlPlaneCR(workshop, r.Scheme, SERVICE_MESH_CONTROL_PLANE_CR_NAME, istioSystemNamespace.Name)
	if err := r.Create(context.TODO(), serviceMeshControlPlaneCR); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Control Plane Custom Resource", serviceMeshControlPlaneCR.Name)
	}

	serviceMeshMemberRollCR := maistra.NewServiceMeshMemberRollCR(workshop, r.Scheme,
		SERVICE_MESH_MEMBER_ROLL_CR_NAME, istioSystemNamespace.Name, istioMembers)
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
				log.Infof("Updated %s Member Roll Custom Resource", serviceMeshMemberRollCRFound.Name)
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

	redhatOperatorsNamespace := kubernetes.NewNamespace(workshop, r.Scheme, REDHAT_OPERATOR_NAMESPACE_NAME)
	if err := r.Create(context.TODO(), redhatOperatorsNamespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Namespace", redhatOperatorsNamespace.Name)
	}

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, subcriptionName, ELASTICSEARCH_SUBSCRIPTION_NAMESPACE_NAME,
		ELASTICSEARCH_PACKAGE_NAME, channel, clusterserviceversion)
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
		JAEGER_PACKAGE_NAME, channel, clusterserviceversion)
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
		KIALI_PACKAGE_NAME, channel, clusterserviceversion)
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

	servicemeshCSV, JaegerCSV, kialiCSV, err := r.getoperatorCSV(workshop)
	if err != nil {
		log.Error("Failed to get ClusterServiceVersion")
	}

	if result, err := r.deleteServiceMesh(workshop, userID); util.IsRequeued(result, err) {
		return result, err
	}
	if result, err := r.deleteKialiOperator(workshop); util.IsRequeued(result, err) {
		return result, err
	}
	if result, err := r.deleteJaegerOperator(workshop); util.IsRequeued(result, err) {
		return result, err
	}

	if result, err := r.deleteElasticSearchOperator(workshop); util.IsRequeued(result, err) {
		return result, err
	}

	if result, err := r.deleteServiceMeshServiceNamespace(workshop); util.IsRequeued(result, err) {
		return result, err
	}

	if result, err := r.deleteOperatorCSV(workshop, servicemeshCSV, kialiCSV, JaegerCSV); util.IsRequeued(result, err) {
		return result, err
	}

	return reconcile.Result{}, nil
}

// Delete ServiceMesh
func (r *WorkshopReconciler) deleteServiceMesh(workshop *workshopv1.Workshop, users int) (reconcile.Result, error) {

	// Service Mesh Operator
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

	istioSystemNamespace := kubernetes.NewNamespace(workshop, r.Scheme, ISTIO_NAMESPACE_NAME)

	jaegerRole := kubernetes.NewRole(workshop, r.Scheme,
		JAEGER_ROLE_NAME, JAEGER_ROLE_NAMESPACE_NAME, IstioLabels, kubernetes.JaegerUserRules())

	serviceMeshMemberRollCR := maistra.NewServiceMeshMemberRollCR(workshop, r.Scheme,
		SERVICE_MESH_MEMBER_ROLL_CR_NAME, istioSystemNamespace.Name, istioMembers)
	// Delete service MeshMember Roll CR
	if err := r.Delete(context.TODO(), serviceMeshMemberRollCR); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Custom Resource", serviceMeshMemberRollCR.Name)

	serviceMeshControlPlaneCR := maistra.NewServiceMeshControlPlaneCR(workshop, r.Scheme, SERVICE_MESH_CONTROL_PLANE_CR_NAME, istioSystemNamespace.Name)
	// Delete Service Mesh Control Plane Custom Resource
	if err := r.Delete(context.TODO(), serviceMeshControlPlaneCR); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Control Plane Custom Resource", serviceMeshControlPlaneCR.Name)

	meshUserRoleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme,
		SERVICE_MESH_ROLE_BINDING_NAME, SERVICE_MESH_ROLE_BINDING_NAMESPACE_NAME, IstioLabels, istioUsers, SERVICE_MESH_ROLE_NAME, SERVICE_MESH_ROLE_KIND_NAME)
	// Delete meshUser RoleBinding
	if err := r.Delete(context.TODO(), meshUserRoleBinding); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Role Binding", meshUserRoleBinding.Name)

	jaegerRoleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme,
		JAEGER_ROLE_BINDING_NAME, JAEGER_ROLE_BINDING_NAMESPACE_NAME, IstioLabels, istioUsers, jaegerRole.Name, JAEGER_ROLE_KIND_NAME)
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
		SERVICE_MESH_PACKAGE_NAME, channel, clusterserviceversion)
	// Delete Subscription
	if err := r.Delete(context.TODO(), subscription); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Subscription", subscription.Name)

	//Success
	return reconcile.Result{}, nil
}

// Delete KialiOperator
func (r *WorkshopReconciler) deleteKialiOperator(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.ServiceMesh.KialiOperatorHub.Channel
	clusterserviceversion := workshop.Spec.Infrastructure.ServiceMesh.KialiOperatorHub.ClusterServiceVersion

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, KIALI_SUBSCRIPTION_NAME, KIALI_SUBSCRIPTION_NAMESPACE_NAME,
		KIALI_PACKAGE_NAME, channel, clusterserviceversion)
	// Delete Subscription
	if err := r.Delete(context.TODO(), subscription); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Subscription", subscription.Name)

	//Success
	return reconcile.Result{}, nil
}

// Delete JaegerOperator
func (r *WorkshopReconciler) deleteJaegerOperator(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.ServiceMesh.JaegerOperatorHub.Channel
	clusterserviceversion := workshop.Spec.Infrastructure.ServiceMesh.JaegerOperatorHub.ClusterServiceVersion

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, JAEGER_SUBSCRIPTION_NAME, JAEGER_SUBSCRIPTION_NAMESPACE_NAME,
		JAEGER_PACKAGE_NAME, channel, clusterserviceversion)
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
		ELASTICSEARCH_PACKAGE_NAME, channel, clusterserviceversion)
	// Delete Subscription
	if err := r.Delete(context.TODO(), subscription); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Subscription", subscription.Name)
	//Success
	return reconcile.Result{}, nil
}

func (r *WorkshopReconciler) deleteServiceMeshServiceNamespace(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	istioSystemNamespace := kubernetes.NewNamespace(workshop, r.Scheme, ISTIO_NAMESPACE_NAME)
	// Delete istioSystem Namespace
	if err := r.Delete(context.TODO(), istioSystemNamespace); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Namespace", istioSystemNamespace.Name)

	redhatOperatorsNamespace := kubernetes.NewNamespace(workshop, r.Scheme, REDHAT_OPERATOR_NAMESPACE_NAME)
	// Delete Namespace
	if err := r.Delete(context.TODO(), redhatOperatorsNamespace); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Namespace", redhatOperatorsNamespace.Name)

	//Success
	return reconcile.Result{}, nil
}

func (r *WorkshopReconciler) getoperatorCSV(workshop *workshopv1.Workshop) (string, string, string, error) {

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

func (r *WorkshopReconciler) deleteOperatorCSV(workshop *workshopv1.Workshop, servicemeshCSV string, kialiCSV string, JaegerCSV string) (reconcile.Result, error) {
	log.Info("start deleteOperatorCSV method ")

	servicemeshOperatorCSV := kubernetes.NewRedHatClusterServiceVersion(workshop, r.Scheme, servicemeshCSV, SERVICE_MESH_SUBSCRIPTION_NAMESPACE_NAME)
	if err := r.Delete(context.TODO(), servicemeshOperatorCSV); err != nil {
		log.Error("ServicemeshOperatorCSV not delete")
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s ClusterServiceVersion", servicemeshOperatorCSV.Name)

	kialiOperatorCSV := kubernetes.NewRedHatClusterServiceVersion(workshop, r.Scheme, kialiCSV, KIALI_SUBSCRIPTION_NAMESPACE_NAME)
	if err := r.Delete(context.TODO(), kialiOperatorCSV); err != nil {
		log.Error("KialiOperatorCSV not delete")
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s ClusterServiceVersion", kialiOperatorCSV.Name)

	JaegerOperatorCSV := kubernetes.NewRedHatClusterServiceVersion(workshop, r.Scheme, JaegerCSV, JAEGER_SUBSCRIPTION_NAMESPACE_NAME)
	if err := r.Delete(context.TODO(), JaegerOperatorCSV); err != nil {
		log.Error("JaegerOperatorCSV not delete")
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s ClusterServiceVersion", JaegerOperatorCSV.Name)

	return reconcile.Result{}, nil
}