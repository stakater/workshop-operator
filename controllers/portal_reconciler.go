package controllers

import (
	"context"
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

const (
	REDISPERSISTENTVOLUMECLAIM = "512Mi"
	REDISDEPLOYMENTNAME        = "redis"
	REDISSERVICENAME           = "redis"
	PORTALSERVICENAME          = "portal"
	REDISROUTEPORT             = 8080
)

var redislabels = map[string]string{
	"app":                       REDISSERVICENAME,
	"app.kubernetes.io/part-of": "portal",
}

var rediscredentials = map[string]string{
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
	secret := kubernetes.NewStringDataSecret(workshop, r.Scheme, REDISSERVICENAME, workshop.Namespace, redislabels, rediscredentials)
	if err := r.Create(context.TODO(), secret); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Secret", secret.Name)
	}

	persistentVolumeClaim := kubernetes.NewPersistentVolumeClaim(workshop, r.Scheme, REDISSERVICENAME, workshop.Namespace, redislabels, REDISPERSISTENTVOLUMECLAIM)
	if err := r.Create(context.TODO(), persistentVolumeClaim); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Persistent Volume Claim", persistentVolumeClaim.Name)
	}

	// Deploy/Update UsernameDistribution
	dep := redis.NewDeployment(workshop, r.Scheme, REDISDEPLOYMENTNAME, workshop.Namespace, redislabels)
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
	service := kubernetes.NewService(workshop, r.Scheme, REDISSERVICENAME, workshop.Namespace, redislabels, []string{"http"}, []int32{6379})
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
	log.Info("Creating UpdateUsernameDistribution")
	// Deploy/Update UsernameDistribution
	dep := usernamedistribution.NewDeployment(workshop, r.Scheme, PORTALSERVICENAME, redislabels, REDISSERVICENAME, users, appsHostnameSuffix, openshiftConsoleURL)
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
	service := kubernetes.NewService(workshop, r.Scheme, PORTALSERVICENAME, workshop.Namespace, redislabels, []string{"http"}, []int32{8080})
	if err := r.Create(context.TODO(), service); err != nil && !errors.IsAlreadyExists(err) {
		return reconcile.Result{}, err
	} else if err == nil {
		log.Infof("Created %s Service", service.Name)
	}

	// Create Route
	route := kubernetes.NewSecuredRoute(workshop, r.Scheme, PORTALSERVICENAME, workshop.Namespace, redislabels, PORTALSERVICENAME, int32(REDISROUTEPORT))
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
	log.Info("Deleting Redis")
	service := kubernetes.NewService(workshop, r.Scheme, REDISSERVICENAME, workshop.Namespace, redislabels, []string{"http"}, []int32{6379})
	// Delete Service
	if err := r.Delete(context.TODO(), service); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Service", service.Name)

	dep := redis.NewDeployment(workshop, r.Scheme, REDISDEPLOYMENTNAME, workshop.Namespace, redislabels)
	// Delete Deployment
	if err := r.Delete(context.TODO(), dep); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Deployment ", dep.Name)

	persistentVolumeClaim := kubernetes.NewPersistentVolumeClaim(workshop, r.Scheme, REDISSERVICENAME, workshop.Namespace, redislabels, REDISPERSISTENTVOLUMECLAIM)
	// Delete persistentVolume Claim
	if err := r.Delete(context.TODO(), persistentVolumeClaim); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Persistent Volume Claim", persistentVolumeClaim.Name)

	secret := kubernetes.NewStringDataSecret(workshop, r.Scheme, REDISSERVICENAME, workshop.Namespace, redislabels, rediscredentials)
	// Delete secret
	if err := r.Delete(context.TODO(), secret); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Secret", secret.Name)
	log.Info("Redis deleted successfully")

	//Success
	return reconcile.Result{}, nil
}

// delete UsernameDistribution
func (r *WorkshopReconciler) deleteUpdateUsernameDistribution(workshop *workshopv1.Workshop,
	users int, appsHostnameSuffix string, openshiftConsoleURL string) (reconcile.Result, error) {
	log.Info("Deleting gitea UpdateUsernameDistribution ")
	route := kubernetes.NewSecuredRoute(workshop, r.Scheme, PORTALSERVICENAME, workshop.Namespace, redislabels, PORTALSERVICENAME, int32(REDISROUTEPORT))
	// Delete Route
	if err := r.Delete(context.TODO(), route); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Route", route.Name)

	service := kubernetes.NewService(workshop, r.Scheme, PORTALSERVICENAME, workshop.Namespace, redislabels, []string{"http"}, []int32{8080})
	// Delete Service
	if err := r.Delete(context.TODO(), service); err != nil {
		return reconcile.Result{}, err
	}
	log.Infof("Deleted %s Service", service.Name)

	dep := usernamedistribution.NewDeployment(workshop, r.Scheme, PORTALSERVICENAME, redislabels, REDISSERVICENAME, users, appsHostnameSuffix, openshiftConsoleURL)
	deploymentFound := &appsv1.Deployment{}
	deploymentErr := r.Get(context.TODO(), types.NamespacedName{Name: dep.Name, Namespace: workshop.Namespace}, deploymentFound)
	if deploymentErr == nil {
		// Delete Deployment
		if err := r.Delete(context.TODO(), dep); err != nil {
			return reconcile.Result{}, err
		}
		log.Infof("Deleted %s Deployment", dep.Name)
	}
	log.Info("UpdateUsernameDistribution deleted successfully")
	//Success
	return reconcile.Result{}, nil
}
