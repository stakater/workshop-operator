package controllers

import (
	"context"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	"reflect"

	"github.com/prometheus/common/log"
	"github.com/stakater/workshop-operator/common/kubernetes"
	"github.com/stakater/workshop-operator/common/redis"
	"github.com/stakater/workshop-operator/common/usernamedistribution"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	"github.com/stakater/workshop-operator/common/util"
)

// reconcilePortal reconciles Portal
func (r *WorkshopReconciler) reconcilePortal(workshop *workshopv1.Workshop, users int,
	appsHostnameSuffix string, openshiftConsoleURL string) (reconcile.Result, error) {

	if result, err := r.addRedis(workshop); util.IsRequeued(result, err) {
		return result, err
	}

	if result, err := r.addUpdateUsernameDistribution(workshop, users, appsHostnameSuffix, openshiftConsoleURL); err != nil {
		return result, err
	}

	if result, err := r.deleteRedis(workshop); util.IsRequeued(result, err) {
		return result, err
	}
	if result, err := r.deleteUpdateUsernameDistribution(workshop, users, appsHostnameSuffix, openshiftConsoleURL); err != nil {
		return result, err
	}

	//Success
	return reconcile.Result{}, nil
}

func (r *WorkshopReconciler) addRedis(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	serviceName := "redis"
	labels := map[string]string{
		"app":                       serviceName,
		"app.kubernetes.io/part-of": "portal",
	}

	credentials := map[string]string{
		"database-password": "redis",
	}
	secret := kubernetes.NewStringDataSecret(workshop, r.Scheme, serviceName, workshop.Namespace, labels, credentials)
	if err := r.Create(context.TODO(), secret); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Secret", secret.Name)
	}

	persistentVolumeClaim := kubernetes.NewPersistentVolumeClaim(workshop, r.Scheme, serviceName, workshop.Namespace, labels, "512Mi")
	if err := r.Create(context.TODO(), persistentVolumeClaim); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Persistent Volume Claim", persistentVolumeClaim.Name)
	}

	// Deploy/Update UsernameDistribution
	dep := redis.NewDeployment(workshop, r.Scheme, "redis", workshop.Namespace, labels)
	if err := r.Create(context.TODO(), dep); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Deployment", dep.Name)
	} else if errors.IsAlreadyExists(err) {
		deploymentFound := &appsv1.Deployment{}
		if err := r.Get(context.TODO(), types.NamespacedName{Name: dep.Name, Namespace: workshop.Namespace}, deploymentFound); err != nil {
			return reconcile.Result{}, err
		} else if err == nil {
			if !reflect.DeepEqual(dep.Spec.Template.Spec.Containers[0].Env, deploymentFound.Spec.Template.Spec.Containers[0].Env) {
				// Update Guide
				if err := r.Update(context.TODO(), dep); err != nil {
					return reconcile.Result{}, err
				}
				log.Infof("Updated %s Deployment", dep.Name)
			}
		}
	}

	// Create Service
	service := kubernetes.NewService(workshop, r.Scheme, serviceName, workshop.Namespace, labels, []string{"http"}, []int32{6379})
	if err := r.Create(context.TODO(), service); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Service", service.Name)
	}

	//Success
	return reconcile.Result{}, nil
}

func (r *WorkshopReconciler) addUpdateUsernameDistribution(workshop *workshopv1.Workshop,
	users int, appsHostnameSuffix string, openshiftConsoleURL string) (reconcile.Result, error) {

	serviceName := "portal"
	redisServiceName := "redis"
	labels := map[string]string{
		"app":                       serviceName,
		"app.kubernetes.io/part-of": "portal",
	}

	// Deploy/Update UsernameDistribution
	dep := usernamedistribution.NewDeployment(workshop, r.Scheme, serviceName, labels, redisServiceName, users, appsHostnameSuffix, openshiftConsoleURL)
	if err := r.Create(context.TODO(), dep); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Deployment", dep.Name)
	} else if errors.IsAlreadyExists(err) {
		deploymentFound := &appsv1.Deployment{}
		if err := r.Get(context.TODO(), types.NamespacedName{Name: dep.Name, Namespace: workshop.Namespace}, deploymentFound); err != nil {
			return reconcile.Result{}, err
		} else if err == nil {
			if !reflect.DeepEqual(dep.Spec.Template.Spec.Containers[0].Env, deploymentFound.Spec.Template.Spec.Containers[0].Env) ||
				!reflect.DeepEqual(dep.Spec.Template.Spec.Containers[0].Image, deploymentFound.Spec.Template.Spec.Containers[0].Image) {
				// Update Guide
				if err := r.Update(context.TODO(), dep); err != nil {
					return reconcile.Result{}, err
				}
				log.Infof("Updated %s Deployment", dep.Name)
			}
		}
	}

	// Create Service
	service := kubernetes.NewService(workshop, r.Scheme, serviceName, workshop.Namespace, labels, []string{"http"}, []int32{8080})
	if err := r.Create(context.TODO(), service); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Service", service.Name)
	}

	// Create Route
	route := kubernetes.NewSecuredRoute(workshop, r.Scheme, serviceName, workshop.Namespace, labels, serviceName, 8080)
	if err := r.Create(context.TODO(), route); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Route", route.Name)
	}

	//Success
	return reconcile.Result{}, nil
}

// delete Redis
func (r *WorkshopReconciler) deleteRedis(workshop *workshopv1.Workshop) (reconcile.Result, error) {

	serviceName := "redis"
	labels := map[string]string{
		"app":                       serviceName,
		"app.kubernetes.io/part-of": "portal",
	}

	credentials := map[string]string{
		"database-password": "redis",
	}


	service := kubernetes.NewService(workshop, r.Scheme, serviceName, workshop.Namespace, labels, []string{"http"}, []int32{6379})
	serviceFound := &corev1.Service{}
	serviceErr := r.Get(context.TODO(), types.NamespacedName{Name: service.Name, Namespace: service.Namespace},serviceFound )
	if serviceErr == nil {
		// Delete Service
		if err := r.Delete(context.TODO(),service); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Service", service.Name)
	}

	dep := redis.NewDeployment(workshop, r.Scheme, "redis", workshop.Namespace, labels)
	deploymentFound := &appsv1.Deployment{}
	deploymentErr := r.Get(context.TODO(), types.NamespacedName{Name: dep.Name, Namespace: workshop.Namespace}, deploymentFound)
	if deploymentErr == nil{
		// Delete Deployment
		if err := r.Delete(context.TODO(),dep); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Deployment ", dep.Name)
	}

	persistentVolumeClaim := kubernetes.NewPersistentVolumeClaim(workshop, r.Scheme, serviceName, workshop.Namespace, labels, "512Mi")
	persistentVolumeClaimFound := &corev1.PersistentVolumeClaim{}
	persistentVolumeClaimErr := r.Get(context.TODO(), types.NamespacedName{Name: serviceName, Namespace: workshop.Namespace},persistentVolumeClaimFound )
	if persistentVolumeClaimErr == nil {
		// Delete persistentVolume Claim
		if err := r.Delete(context.TODO(),persistentVolumeClaim); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Persistent Volume Claim", persistentVolumeClaim.Name)
	}

	secret := kubernetes.NewStringDataSecret(workshop, r.Scheme, serviceName, workshop.Namespace, labels, credentials)
	secretFound := &corev1.Secret{}
	secretErr := r.Get(context.TODO(), types.NamespacedName{Name: serviceName, Namespace: workshop.Namespace}, secretFound)
	if secretErr == nil {
		// Delete secret
		if err := r.Delete(context.TODO(), secret); err != nil{
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Secret", secret.Name)
	}

	//Success
	return reconcile.Result{}, nil
}

// delete UsernameDistribution
func (r *WorkshopReconciler) deleteUpdateUsernameDistribution(workshop *workshopv1.Workshop,
	users int, appsHostnameSuffix string, openshiftConsoleURL string) (reconcile.Result, error) {

	serviceName := "portal"
	redisServiceName := "redis"
	labels := map[string]string{
		"app":                       serviceName,
		"app.kubernetes.io/part-of": "portal",
	}

	route := kubernetes.NewSecuredRoute(workshop, r.Scheme, serviceName, workshop.Namespace, labels, serviceName, 8080)
	routeFound := &routev1.Route{}
	routeErr := r.Get(context.TODO(), types.NamespacedName{Name: route.Name,Namespace: workshop.Namespace},routeFound )
	if routeErr == nil {
		// Delete Route
		if err := r.Delete(context.TODO(),route); err != nil{
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Route", route.Name)
	}

	service := kubernetes.NewService(workshop, r.Scheme, serviceName, workshop.Namespace, labels, []string{"http"}, []int32{8080})
	serviceFound := &corev1.Service{}
	serviceErr := r.Get(context.TODO(), types.NamespacedName{Name:service.Name , Namespace:workshop.Namespace }, serviceFound)
	if serviceErr == nil {
		// Delete Service
		if err := r.Delete(context.TODO(),service ); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Service", service.Name)
	}

	dep := usernamedistribution.NewDeployment(workshop, r.Scheme, serviceName, labels, redisServiceName, users, appsHostnameSuffix, openshiftConsoleURL)
	deploymentFound := &appsv1.Deployment{}
	deploymentErr := r.Get(context.TODO(), types.NamespacedName{Name: dep.Name, Namespace: workshop.Namespace}, deploymentFound)
	if deploymentErr == nil {
		// Delete Deployment
		if err := r.Delete(context.TODO(),dep); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Deployment", dep.Name)
	}
	//Success
	return reconcile.Result{}, nil
}