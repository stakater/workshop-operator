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
	ELASTICSEARCHOPERATORNAMESPACENAME    = "openshift-operators-redhat"
	ELASTICSEARCHOPERATORSUBSCRIPTIONNAME = "openshift-operators-redhat"
	ELASTICSEARCHOPERATORPACKAGENAME      = "elasticsearch-operator"
	JAEGEROPERATORNAMESPACENAME           = "openshift-operators"
	JAEGEROPERATORNAME                    = "jaeger-product"
	JAEGEROPERATORNAMEPACKAGENAME         = "jaeger-product"
	KIALIOPERATORNAMESPACENAME            = "openshift-operators"
	KIALIOPERATORNAME                     = "kiali-ossm"
	KIALIOPERATORPACKAGENAME              = "kiali-ossm"
	OPERATORNAMESPACE                     = "openshift-operators"
	SERVICEMESHPACKAGENAME                = "servicemeshoperator"
	SERVICEMESHSUBSCRIPTIONNAME           = "servicemeshoperator"
	JAEGERROLENAME                        = "jaeger-user"
	JAEGERROLENAMESPACENAME               = "istio-system"
	ISTIONAMESPACENAME                    = "istio-system"
	JAEGERROLEROLEBINDINGNAME             = "jaeger-users"
	JAEGERROLEROLEBINDINGNAMESPACENAME    = "istio-system"
	JAEGERROLEROLEBINDINGKINDNAME         = "Role"
	MESHROLEBINDINGNAME                   = "mesh-users"
	MESHROLEBINDINGNAMESPACENAME          = "istio-system"
	MESHROLEBINDINGROLENAME               = "mesh-user"
	MESHROLEBINDINGKINDNAME               = "Role"
	SERVICEMESHMEMBERROLLCRNAME           = "default"
	SERVICEMESHCONTROLPLANECRNAME         = "basic"
	SERVICEMESHDEPLOYMENTSTATUSNAME       = "istio-operator"
)

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

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, SERVICEMESHSUBSCRIPTIONNAME, OPERATORNAMESPACE,
		SERVICEMESHPACKAGENAME, channel, clusterserviceversion)
	if err := r.Create(context.TODO(), subscription); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s ServiceMesh Subscription", subscription.Name)
	}

	if err := r.ApproveInstallPlan(clusterserviceversion, SERVICEMESHSUBSCRIPTIONNAME, OPERATORNAMESPACE); err != nil {
		log.Infof("Waiting for ServiceMesh Subscription to create InstallPlan for %s", subscription.Name)
		return reconcile.Result{Requeue: true}, nil
	}

	// Wait for Operator to be running
	if !kubernetes.GetK8Client().GetDeploymentStatus(SERVICEMESHDEPLOYMENTSTATUSNAME, OPERATORNAMESPACE) {
		return reconcile.Result{Requeue: true}, nil
	}

	// Deploy Service Mesh
	istioSystemNamespace := kubernetes.NewNamespace(workshop, r.Scheme, ISTIONAMESPACENAME)
	if err := r.Create(context.TODO(), istioSystemNamespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s istio Namespace", istioSystemNamespace.Name)
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

	labels := map[string]string{
		"app.kubernetes.io/part-of": "istio",
	}

	jaegerRole := kubernetes.NewRole(workshop, r.Scheme,
		JAEGERROLENAME, JAEGERROLENAMESPACENAME, labels, kubernetes.JaegerUserRules())
	if err := r.Create(context.TODO(), jaegerRole); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s jaeger Role", jaegerRole.Name)
	}

	jaegerRoleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme,
		JAEGERROLEROLEBINDINGNAME, JAEGERROLEROLEBINDINGNAMESPACENAME, labels, istioUsers, jaegerRole.Name, JAEGERROLEROLEBINDINGKINDNAME)
	if err := r.Create(context.TODO(), jaegerRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s jaeger Role Binding", jaegerRoleBinding.Name)
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
				log.Infof("Updated %s jaeger Role Binding", found.Name)
			}
		}
	}

	meshUserRoleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme,
		MESHROLEBINDINGNAME, MESHROLEBINDINGNAMESPACENAME, labels, istioUsers, MESHROLEBINDINGROLENAME, MESHROLEBINDINGKINDNAME)

	if err := r.Create(context.TODO(), meshUserRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Mesh Role Binding", meshUserRoleBinding.Name)
	}

	serviceMeshControlPlaneCR := maistra.NewServiceMeshControlPlaneCR(workshop, r.Scheme, SERVICEMESHCONTROLPLANECRNAME, istioSystemNamespace.Name)
	if err := r.Create(context.TODO(), serviceMeshControlPlaneCR); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Service Mesh Control Plane Custom Resource", serviceMeshControlPlaneCR.Name)
	}

	serviceMeshMemberRollCR := maistra.NewServiceMeshMemberRollCR(workshop, r.Scheme,
		SERVICEMESHMEMBERROLLCRNAME, istioSystemNamespace.Name, istioMembers)
	if err := r.Create(context.TODO(), serviceMeshMemberRollCR); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Service Mesh Custom Resource", serviceMeshMemberRollCR.Name)
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

	redhatOperatorsNamespace := kubernetes.NewNamespace(workshop, r.Scheme, ELASTICSEARCHOPERATORNAMESPACENAME)
	if err := r.Create(context.TODO(), redhatOperatorsNamespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s ElasticSearchOperator Namespace", redhatOperatorsNamespace.Name)
	}

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, subcriptionName, ELASTICSEARCHOPERATORSUBSCRIPTIONNAME,
		ELASTICSEARCHOPERATORPACKAGENAME, channel, clusterserviceversion)
	if err := r.Create(context.TODO(), subscription); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s ElasticSearchOperator Subscription", subscription.Name)
	}

	if err := r.ApproveInstallPlan(clusterserviceversion, subcriptionName, ELASTICSEARCHOPERATORSUBSCRIPTIONNAME); err != nil {
		log.Infof("Waiting for ElasticSearchOperator Subscription to create InstallPlan for %s", subscription.Name)
		return reconcile.Result{Requeue: true}, nil
	}

	//Success
	return reconcile.Result{}, nil
}

// Add JaegerOperator
func (r *WorkshopReconciler) addJaegerOperator(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.ServiceMesh.JaegerOperatorHub.Channel
	clusterserviceversion := workshop.Spec.Infrastructure.ServiceMesh.JaegerOperatorHub.ClusterServiceVersion

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, JAEGEROPERATORNAME, JAEGEROPERATORNAMESPACENAME,
		JAEGEROPERATORNAMEPACKAGENAME, channel, clusterserviceversion)
	if err := r.Create(context.TODO(), subscription); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s jaeger Subscription", subscription.Name)
	}

	if err := r.ApproveInstallPlan(clusterserviceversion, JAEGEROPERATORNAME, JAEGEROPERATORNAMESPACENAME); err != nil {
		log.Infof("Waiting for jaeger Subscription to create InstallPlan for %s", subscription.Name)
		return reconcile.Result{Requeue: true}, nil
	}

	//Success
	return reconcile.Result{}, nil
}

// Add KialiOperator
func (r *WorkshopReconciler) addKialiOperator(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.ServiceMesh.KialiOperatorHub.Channel
	clusterserviceversion := workshop.Spec.Infrastructure.ServiceMesh.KialiOperatorHub.ClusterServiceVersion

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, KIALIOPERATORNAME, KIALIOPERATORNAMESPACENAME,
		KIALIOPERATORPACKAGENAME, channel, clusterserviceversion)
	if err := r.Create(context.TODO(), subscription); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s kiali Subscription", subscription.Name)
	}

	if err := r.ApproveInstallPlan(clusterserviceversion, KIALIOPERATORNAME, KIALIOPERATORNAMESPACENAME); err != nil {
		log.Infof("Waiting for kiali Subscription to create InstallPlan for %s", subscription.Name)
		return reconcile.Result{Requeue: true}, nil
	}

	//Success
	return reconcile.Result{}, nil
}

// Delete ServiceMesh
func (r *WorkshopReconciler) deleteServiceMesh(workshop *workshopv1.Workshop, users int) (reconcile.Result, error) {

	log.Infoln("Deleting ServiceMesh")

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

	labels := map[string]string{
		"app.kubernetes.io/part-of": "istio",
	}
	istioSystemNamespace := kubernetes.NewNamespace(workshop, r.Scheme, ISTIONAMESPACENAME)

	jaegerRole := kubernetes.NewRole(workshop, r.Scheme,
		JAEGERROLENAME, JAEGERROLENAMESPACENAME, labels, kubernetes.JaegerUserRules())

	serviceMeshMemberRollCR := maistra.NewServiceMeshMemberRollCR(workshop, r.Scheme,
		SERVICEMESHMEMBERROLLCRNAME, istioSystemNamespace.Name, istioMembers)
	// Delete service MeshMember Roll CR
	if err := r.Delete(context.TODO(), serviceMeshMemberRollCR); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Service Mesh Custom Resource", serviceMeshMemberRollCR.Name)

	serviceMeshControlPlaneCR := maistra.NewServiceMeshControlPlaneCR(workshop, r.Scheme, SERVICEMESHCONTROLPLANECRNAME, istioSystemNamespace.Name)
	// Delete Service Mesh Control Plane Custom Resource
	if err := r.Delete(context.TODO(), serviceMeshControlPlaneCR); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Service Mesh Control Plane Custom Resource", serviceMeshControlPlaneCR.Name)

	meshUserRoleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme,
		MESHROLEBINDINGNAME, MESHROLEBINDINGNAMESPACENAME, labels, istioUsers, MESHROLEBINDINGROLENAME, MESHROLEBINDINGKINDNAME)
	// Delete meshUser RoleBinding
	if err := r.Delete(context.TODO(), meshUserRoleBinding); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Mesh Role Binding", meshUserRoleBinding.Name)

	jaegerRoleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme,
		JAEGERROLEROLEBINDINGNAME, JAEGERROLEROLEBINDINGNAMESPACENAME, labels, istioUsers, jaegerRole.Name, JAEGERROLEROLEBINDINGKINDNAME)
	// Delete jaeger RoleBinding
	if err := r.Delete(context.TODO(), jaegerRoleBinding); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s jaeger Role Binding", jaegerRoleBinding.Name)

	// Delete jaeger Role
	if err := r.Delete(context.TODO(), jaegerRole); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s jaeger Role", jaegerRole.Name)

	// Delete istioSystem Namespace
	if err := r.Delete(context.TODO(), istioSystemNamespace); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s istio Namespace", istioSystemNamespace.Name)

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, SERVICEMESHSUBSCRIPTIONNAME, OPERATORNAMESPACE,
		SERVICEMESHPACKAGENAME, channel, clusterserviceversion)
	// Delete Subscription
	if err := r.Delete(context.TODO(), subscription); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Subscription", subscription.Name)
	log.Infoln("Deleted Service Mesh Successfully")
	//Success
	return reconcile.Result{}, nil
}

// Delete ElasticSearchOperator1
func (r *WorkshopReconciler) deleteElasticSearchOperator(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	log.Info("Deleting ElasticSearchOperator")
	channel := workshop.Spec.Infrastructure.ServiceMesh.ElasticSearchOperatorHub.Channel
	clusterserviceversion := workshop.Spec.Infrastructure.ServiceMesh.ElasticSearchOperatorHub.ClusterServiceVersion
	subcriptionName := fmt.Sprintf("elasticsearch-operator-%s", channel)

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, subcriptionName, ELASTICSEARCHOPERATORSUBSCRIPTIONNAME,
		ELASTICSEARCHOPERATORPACKAGENAME, channel, clusterserviceversion)
	// Delete Subscription
	if err := r.Delete(context.TODO(), subscription); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s ElasticSearchOperator Subscription", subscription.Name)

	redhatOperatorsNamespace := kubernetes.NewNamespace(workshop, r.Scheme, ELASTICSEARCHOPERATORNAMESPACENAME)
	// Delete Namespace
	if err := r.Delete(context.TODO(), redhatOperatorsNamespace); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s ElasticSearchOperator Namespace", redhatOperatorsNamespace.Name)
	log.Info("Deleted ElasticSearchOperator Successfully")
	//Success
	return reconcile.Result{}, nil
}

// Delete JaegerOperator
func (r *WorkshopReconciler) deleteJaegerOperator(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	log.Infoln("Deleting jaeger")
	channel := workshop.Spec.Infrastructure.ServiceMesh.JaegerOperatorHub.Channel
	clusterserviceversion := workshop.Spec.Infrastructure.ServiceMesh.JaegerOperatorHub.ClusterServiceVersion

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, JAEGEROPERATORNAME, JAEGEROPERATORNAMESPACENAME,
		JAEGEROPERATORNAMEPACKAGENAME, channel, clusterserviceversion)
	// Delete Subscription
	if err := r.Delete(context.TODO(), subscription); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s jaeger Subscription", subscription.Name)
	log.Infoln("Deleted jaeger Successfully")
	//Success
	return reconcile.Result{}, nil
}

// Delete KialiOperator
func (r *WorkshopReconciler) deleteKialiOperator(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	log.Infoln("Deleting KialiOperator")
	channel := workshop.Spec.Infrastructure.ServiceMesh.KialiOperatorHub.Channel
	clusterserviceversion := workshop.Spec.Infrastructure.ServiceMesh.KialiOperatorHub.ClusterServiceVersion

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, KIALIOPERATORNAME, KIALIOPERATORNAMESPACENAME,
		KIALIOPERATORPACKAGENAME, channel, clusterserviceversion)
	subscriptionFound := &olmv1alpha1.Subscription{}
	subscriptionErr := r.Get(context.TODO(), types.NamespacedName{Name: subscription.Name, Namespace: subscription.Namespace}, subscriptionFound)
	if subscriptionErr == nil {
		// Delete Subscription
		if err := r.Delete(context.TODO(), subscription); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s KialiOperator Subscription", subscription.Name)
	}
	log.Infoln("Deleted KialiOperator Successfully")
	//Success
	return reconcile.Result{}, nil
}
