package controllers

import (
	"context"
	"fmt"
	maistrav2 "github.com/maistra/istio-operator/pkg/apis/maistra/v2"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
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

	if enabledServiceMesh || enabledServerless {

		if result, err := r.deleteElasticSearchOperator(workshop); util.IsRequeued(result, err) {
			return result, err
		}

		if result, err := r.deleteJaegerOperator(workshop); util.IsRequeued(result, err) {
			return result, err
		}

		if result, err := r.deleteKialiOperator(workshop); util.IsRequeued(result, err) {
			return result, err
		}

		if result, err := r.deleteServiceMesh(workshop, users); util.IsRequeued(result, err) {
			return result, err
		}
	}
	//Success
	return reconcile.Result{}, nil
}

// Add ServiceMesh
func (r *WorkshopReconciler) addServiceMesh(workshop *workshopv1.Workshop, users int) (reconcile.Result, error) {

	operatorNamespace := "openshift-operators"

	// Service Mesh Operator
	channel := workshop.Spec.Infrastructure.ServiceMesh.ServiceMeshOperatorHub.Channel
	clusterserviceversion := workshop.Spec.Infrastructure.ServiceMesh.ServiceMeshOperatorHub.ClusterServiceVersion

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, "servicemeshoperator", operatorNamespace,
		"servicemeshoperator", channel, clusterserviceversion)
	if err := r.Create(context.TODO(), subscription); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Subscription", subscription.Name)
	}

	if err := r.ApproveInstallPlan(clusterserviceversion, "servicemeshoperator", operatorNamespace); err != nil {
		log.Infof("Waiting for Subscription to create InstallPlan for %s", subscription.Name)
		return reconcile.Result{Requeue: true}, nil
	}

	// Wait for Operator to be running
	if !kubernetes.GetK8Client().GetDeploymentStatus("istio-operator", operatorNamespace) {
		return reconcile.Result{Requeue: true}, nil
	}

	// Deploy Service Mesh
	istioSystemNamespace := kubernetes.NewNamespace(workshop, r.Scheme, "istio-system")
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

	labels := map[string]string{
		"app.kubernetes.io/part-of": "istio",
	}

	jaegerRole := kubernetes.NewRole(workshop, r.Scheme,
		"jaeger-user", "istio-system", labels, kubernetes.JaegerUserRules())
	if err := r.Create(context.TODO(), jaegerRole); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Role", jaegerRole.Name)
	}

	jaegerRoleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme,
		"jaeger-users", "istio-system", labels, istioUsers, jaegerRole.Name, "Role")
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
		"mesh-users", "istio-system", labels, istioUsers, "mesh-user", "Role")

	if err := r.Create(context.TODO(), meshUserRoleBinding); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Role Binding", meshUserRoleBinding.Name)
	}

	serviceMeshControlPlaneCR := maistra.NewServiceMeshControlPlaneCR(workshop, r.Scheme, "basic", istioSystemNamespace.Name)
	if err := r.Create(context.TODO(), serviceMeshControlPlaneCR); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Service Mesh Control Plane Custom Resource", serviceMeshControlPlaneCR.Name)
	}

	serviceMeshMemberRollCR := maistra.NewServiceMeshMemberRollCR(workshop, r.Scheme,
		"default", istioSystemNamespace.Name, istioMembers)
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

	redhatOperatorsNamespace := kubernetes.NewNamespace(workshop, r.Scheme, "openshift-operators-redhat")
	if err := r.Create(context.TODO(), redhatOperatorsNamespace); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Namespace", redhatOperatorsNamespace.Name)
	}

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, subcriptionName, "openshift-operators-redhat",
		"elasticsearch-operator", channel, clusterserviceversion)
	if err := r.Create(context.TODO(), subscription); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Subscription", subscription.Name)
	}

	if err := r.ApproveInstallPlan(clusterserviceversion, subcriptionName, "openshift-operators-redhat"); err != nil {
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

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, "jaeger-product", "openshift-operators",
		"jaeger-product", channel, clusterserviceversion)
	if err := r.Create(context.TODO(), subscription); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Subscription", subscription.Name)
	}

	if err := r.ApproveInstallPlan(clusterserviceversion, "jaeger-product", "openshift-operators"); err != nil {
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

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, "kiali-ossm", "openshift-operators",
		"kiali-ossm", channel, clusterserviceversion)
	if err := r.Create(context.TODO(), subscription); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Subscription", subscription.Name)
	}

	if err := r.ApproveInstallPlan(clusterserviceversion, "kiali-ossm", "openshift-operators"); err != nil {
		log.Infof("Waiting for Subscription to create InstallPlan for %s", subscription.Name)
		return reconcile.Result{Requeue: true}, nil
	}

	//Success
	return reconcile.Result{}, nil
}

// Delete ServiceMesh
func (r *WorkshopReconciler) deleteServiceMesh(workshop *workshopv1.Workshop, users int) (reconcile.Result, error) {

	operatorNamespace := "openshift-operators"

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
	istioSystemNamespace := kubernetes.NewNamespace(workshop, r.Scheme, "istio-system")

	jaegerRole := kubernetes.NewRole(workshop, r.Scheme,
		"jaeger-user", "istio-system", labels, kubernetes.JaegerUserRules())

	serviceMeshMemberRollCR := maistra.NewServiceMeshMemberRollCR(workshop, r.Scheme,
		"default", istioSystemNamespace.Name, istioMembers)
	serviceMeshMemberRollCRFound := &maistrav1.ServiceMeshMemberRoll{}
	serviceMeshMemberRollCRErr := r.Get(context.TODO(), types.NamespacedName{Name:serviceMeshMemberRollCR.Name , Namespace: istioSystemNamespace.Name},serviceMeshMemberRollCRFound )
	if serviceMeshMemberRollCRErr == nil {
		// Delete service MeshMember Roll CR
		if err := r.Delete(context.TODO(),serviceMeshMemberRollCR ); err!= nil{
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Custom Resource", serviceMeshMemberRollCR.Name)
	}

	serviceMeshControlPlaneCR := maistra.NewServiceMeshControlPlaneCR(workshop, r.Scheme, "basic", istioSystemNamespace.Name)
	serviceMeshControlPlaneCRFound := &maistrav2.ServiceMeshControlPlane{}
	serviceMeshControlPlaneCRErr := r.Get(context.TODO(), types.NamespacedName{Name:serviceMeshControlPlaneCR.Name , Namespace: istioSystemNamespace.Name},serviceMeshControlPlaneCRFound )
	if serviceMeshControlPlaneCRErr == nil {
		// Delete Service Mesh Control Plane Custom Resource
		if err := r.Delete(context.TODO(),serviceMeshControlPlaneCR ); err!= nil{
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Service Mesh Control Plane Custom Resource", serviceMeshControlPlaneCR.Name)
	}

	meshUserRoleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme,
		"mesh-users", "istio-system", labels, istioUsers, "mesh-user", "Role")
	meshUserRoleBindingFound :=&rbac.RoleBinding{}
	meshUserRoleBindingErr := r.Get(context.TODO(), types.NamespacedName{Name:meshUserRoleBinding.Name , Namespace:meshUserRoleBinding.Namespace },meshUserRoleBindingFound )
	if meshUserRoleBindingErr == nil {
		// Delete meshUser RoleBinding
		if err := r.Delete(context.TODO(),meshUserRoleBinding); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Role Binding", meshUserRoleBinding.Name)
	}

	jaegerRoleBinding := kubernetes.NewRoleBindingUsers(workshop, r.Scheme,
		"jaeger-users", "istio-system", labels, istioUsers, jaegerRole.Name, "Role")
	jaegerRoleBindingFound := &rbac.RoleBinding{}
	jaegerRoleBindingErr := r.Get(context.TODO(), types.NamespacedName{Name: jaegerRoleBinding.Name, Namespace: istioSystemNamespace.Name}, jaegerRoleBindingFound)
	if jaegerRoleBindingErr == nil {
		// Delete jaeger RoleBinding
		if err := r.Delete(context.TODO(), jaegerRoleBinding); err != nil{
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Role Binding", jaegerRoleBinding.Name)
	}

	jaegerRoleFound := &rbac.Role{}
	jaegerRoleErr := r.Get(context.TODO(), types.NamespacedName{Name: jaegerRole.Name, Namespace:jaegerRole.Namespace }, jaegerRoleFound)
	if jaegerRoleErr == nil {
		// Delete jaeger Role
		if err := r.Delete(context.TODO(),jaegerRole ); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Role", jaegerRole.Name)
	}

	istioSystemNamespaceFound := &corev1.Namespace{}
	istioSystemNamespaceErr :=  r.Get(context.TODO(), types.NamespacedName{Name: istioSystemNamespace.Name }, istioSystemNamespaceFound)
	if istioSystemNamespaceErr == nil {
		// Delete istioSystem Namespace
		if err := r.Delete(context.TODO(), istioSystemNamespace); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Namespace", istioSystemNamespace.Name)
	}

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, "servicemeshoperator", operatorNamespace,
		"servicemeshoperator", channel, clusterserviceversion)
	subscriptionFound := &olmv1alpha1.Subscription{}
	subscriptionErr := r.Get(context.TODO(), types.NamespacedName{Name:subscription.Name ,Namespace:operatorNamespace },subscriptionFound )
	if subscriptionErr == nil {
		// Delete Subscription
		if err := r.Delete(context.TODO(), subscription); err != nil {
			return reconcile.Result{}, subscriptionErr
		}
		log.Infof("Deleted %s Subscription", subscription.Name)
	}
	//Success
	return reconcile.Result{}, nil
}

// Delete ElasticSearchOperator1
func (r *WorkshopReconciler) deleteElasticSearchOperator(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.ServiceMesh.ElasticSearchOperatorHub.Channel
	clusterserviceversion := workshop.Spec.Infrastructure.ServiceMesh.ElasticSearchOperatorHub.ClusterServiceVersion
	subcriptionName := fmt.Sprintf("elasticsearch-operator-%s", channel)


	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, subcriptionName, "openshift-operators-redhat",
		"elasticsearch-operator", channel, clusterserviceversion)
	subscriptionFound := &olmv1alpha1.Subscription{}
	subscriptionErr := r.Get(context.TODO(), types.NamespacedName{Name:subcriptionName , Namespace:subscription.Namespace }, subscriptionFound)
	if subscriptionErr == nil {
		// Delete Subscription
		if err := r.Delete(context.TODO(),subscription ); err!= nil{
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Subscription", subscription.Name)
	}

	redhatOperatorsNamespace := kubernetes.NewNamespace(workshop, r.Scheme, "openshift-operators-redhat")
	redhatOperatorsNamespaceFound := &corev1.Namespace{}
	redhatOperatorsNamespaceErr := r.Get(context.TODO(), types.NamespacedName{Name:redhatOperatorsNamespace.Name },redhatOperatorsNamespaceFound )
	if redhatOperatorsNamespaceErr == nil {
		// Delete Namespace
		if err := r.Delete(context.TODO(),redhatOperatorsNamespace ); err!= nil{
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Namespace", redhatOperatorsNamespace.Name)
	}
	//Success
	return reconcile.Result{}, nil
}

// Delete JaegerOperator
func (r *WorkshopReconciler) deleteJaegerOperator(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.ServiceMesh.JaegerOperatorHub.Channel
	clusterserviceversion := workshop.Spec.Infrastructure.ServiceMesh.JaegerOperatorHub.ClusterServiceVersion

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, "jaeger-product", "openshift-operators",
		"jaeger-product", channel, clusterserviceversion)
	subscriptionFound := &olmv1alpha1.Subscription{}
	subscriptionErr := r.Get(context.TODO(), types.NamespacedName{Name:subscription.Name , Namespace:subscription.Namespace }, subscriptionFound)
	if subscriptionErr == nil {
		// Delete Subscription
		if err := r.Delete(context.TODO(),subscription ); err!= nil{
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Subscription", subscription.Name)
	}
	//Success
	return reconcile.Result{}, nil
}

// Delete KialiOperator
func (r *WorkshopReconciler) deleteKialiOperator(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	channel := workshop.Spec.Infrastructure.ServiceMesh.KialiOperatorHub.Channel
	clusterserviceversion := workshop.Spec.Infrastructure.ServiceMesh.KialiOperatorHub.ClusterServiceVersion

	subscription := kubernetes.NewRedHatSubscription(workshop, r.Scheme, "kiali-ossm", "openshift-operators",
		"kiali-ossm", channel, clusterserviceversion)
	subscriptionFound := &olmv1alpha1.Subscription{}
	subscriptionErr := r.Get(context.TODO(), types.NamespacedName{Name:subscription.Name , Namespace:subscription.Namespace }, subscriptionFound)
	if subscriptionErr == nil {
		// Delete Subscription
		if err := r.Delete(context.TODO(),subscription ); err!= nil{
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Subscription", subscription.Name)
	}
	//Success
	return reconcile.Result{}, nil
}