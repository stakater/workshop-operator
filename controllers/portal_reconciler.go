package controllers

import (
	"context"
	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	"github.com/stakater/workshop-operator/common/kubernetes"
	"github.com/stakater/workshop-operator/common/redis"
	"github.com/stakater/workshop-operator/common/usernamedistribution"
	"github.com/stakater/workshop-operator/common/util"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	REDIS_VOLUME_SIZE      = "512Mi"
	REDIS_DEPLOYMENT_NAME  = "redis"
	REDIS_SERVICE_NAME     = "redis"
	REDIS_PVC_NAME         = "redis"
	REDIS_SECRET_NAME      = "redis"
	PORTAL_SERVICE_NAME    = "portal"
	PORTAL_DEPLOYMENT_NAME = "portal"
	PORTAL_ROUTE_NAME      = "portal"
	PORTAL_ROUTE_PORT       = 8080
)

var RedisLabels = map[string]string{
	"app":                       REDIS_SERVICE_NAME,
	"app.kubernetes.io/part-of": "portal",
}

var RedisCredentials = map[string]string{
	"database-password": "redis",
}

// reconcilePortal reconciles Portal
func (r *WorkshopReconciler) reconcilePortal(workshop *workshopv1.Workshop, users int,
	appsHostnameSuffix string, openshiftConsoleURL string) (reconcile.Result, error) {

	if result, err := r.addRedis(workshop); util.IsRequeued(result, err) {
		return result, err
	}

	if result, err := r.addUpdateUsernameDistribution(workshop, users, appsHostnameSuffix, openshiftConsoleURL); err != nil {
		return result, err
	}

	//Success
	return reconcile.Result{}, nil
}

func (r *WorkshopReconciler) addRedis(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	log.Info("Creating Redis")
	secret := kubernetes.NewStringDataSecret(workshop, r.Scheme, REDIS_SECRET_NAME, workshop.Namespace, RedisLabels, RedisCredentials)
	if err := r.Create(context.TODO(), secret); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Secret", secret.Name)
	}

	persistentVolumeClaim := kubernetes.NewPersistentVolumeClaim(workshop, r.Scheme, REDIS_PVC_NAME, workshop.Namespace, RedisLabels, REDIS_VOLUME_SIZE)
	if err := r.Create(context.TODO(), persistentVolumeClaim); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Persistent Volume Claim", persistentVolumeClaim.Name)
	}

	// Deploy/Update UsernameDistribution
	dep := redis.NewDeployment(workshop, r.Scheme, REDIS_DEPLOYMENT_NAME, workshop.Namespace, RedisLabels)
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
	service := kubernetes.NewService(workshop, r.Scheme, REDIS_SERVICE_NAME, workshop.Namespace, RedisLabels, []string{"http"}, []int32{6379})
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
	log.Info("Creating portal")
	// Deploy/Update UsernameDistribution
	dep := usernamedistribution.NewDeployment(workshop, r.Scheme, PORTAL_DEPLOYMENT_NAME, RedisLabels, REDIS_SERVICE_NAME, users, appsHostnameSuffix, openshiftConsoleURL)
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
	service := kubernetes.NewService(workshop, r.Scheme, PORTAL_SERVICE_NAME, workshop.Namespace, RedisLabels, []string{"http"}, []int32{8080})
	if err := r.Create(context.TODO(), service); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Service", service.Name)
	}

	// Create Route
	route := kubernetes.NewSecuredRoute(workshop, r.Scheme, PORTAL_ROUTE_NAME, workshop.Namespace, RedisLabels, PORTAL_SERVICE_NAME, int32(PORTAL_ROUTE_PORT))
	if err := r.Create(context.TODO(), route); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Route", route.Name)
	}

	//Success
	return reconcile.Result{}, nil
}

// delete Redis
func (r *WorkshopReconciler) deletePortal(workshop *workshopv1.Workshop, userID int,
	appsHostnameSuffix string, openshiftConsoleURL string) (reconcile.Result, error) {

	if result, err := r.deleteUsernameDistribution(workshop, userID, appsHostnameSuffix, openshiftConsoleURL); util.IsRequeued(result, err) {
		return result, err
	}
	if result, err := r.deleteRedis(workshop); util.IsRequeued(result, err) {
		return result, err
	}

	return reconcile.Result{}, nil
}

// delete Redis
func (r *WorkshopReconciler) deleteRedis(workshop *workshopv1.Workshop) (reconcile.Result, error) {
	log.Info("Deleting Redis")
	service := kubernetes.NewService(workshop, r.Scheme, REDIS_SERVICE_NAME, workshop.Namespace, RedisLabels, []string{"http"}, []int32{6379})
	// Delete Service
	if err := r.Delete(context.TODO(), service); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Service", service.Name)

	dep := redis.NewDeployment(workshop, r.Scheme, REDIS_DEPLOYMENT_NAME, workshop.Namespace, RedisLabels)
	// Delete Deployment
	if err := r.Delete(context.TODO(), dep); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Deployment ", dep.Name)

	persistentVolumeClaim := kubernetes.NewPersistentVolumeClaim(workshop, r.Scheme, REDIS_PVC_NAME, workshop.Namespace, RedisLabels, REDIS_VOLUME_SIZE)
	// Delete persistentVolume Claim
	if err := r.Delete(context.TODO(), persistentVolumeClaim); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Persistent Volume Claim", persistentVolumeClaim.Name)

	secret := kubernetes.NewStringDataSecret(workshop, r.Scheme, REDIS_SECRET_NAME, workshop.Namespace, RedisLabels, RedisCredentials)
	// Delete secret
	if err := r.Delete(context.TODO(), secret); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Secret", secret.Name)
	log.Info("Deleted Redis successfully")

	//Success
	return reconcile.Result{}, nil
}

// delete UsernameDistribution
func (r *WorkshopReconciler) deleteUsernameDistribution(workshop *workshopv1.Workshop,
	users int, appsHostnameSuffix string, openshiftConsoleURL string) (reconcile.Result, error) {

	log.Info("Deleting  portal ")
	route := kubernetes.NewSecuredRoute(workshop, r.Scheme, PORTAL_ROUTE_NAME, workshop.Namespace, RedisLabels, PORTAL_SERVICE_NAME, int32(PORTAL_ROUTE_PORT))
	// Delete Route
	if err := r.Delete(context.TODO(), route); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Route", route.Name)

	service := kubernetes.NewService(workshop, r.Scheme, PORTAL_SERVICE_NAME, workshop.Namespace, RedisLabels, []string{"http"}, []int32{8080})
	// Delete Service
	if err := r.Delete(context.TODO(), service); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Service", service.Name)

	dep := usernamedistribution.NewDeployment(workshop, r.Scheme, PORTAL_DEPLOYMENT_NAME, RedisLabels, REDIS_SERVICE_NAME, users, appsHostnameSuffix, openshiftConsoleURL)
	deploymentFound := &appsv1.Deployment{}
	deploymentErr := r.Get(context.TODO(), types.NamespacedName{Name: dep.Name, Namespace: workshop.Namespace}, deploymentFound)
	if deploymentErr == nil {
		// Delete Deployment
		if err := r.Delete(context.TODO(), dep); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Deployment", dep.Name)
	}
	log.Info(" Deleted portal  successfully")
	//Success
	return reconcile.Result{}, nil
}
