package kubernetes

import (
	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)

// NewService create a service
func NewService(workshop *workshopv1.Workshop, scheme *runtime.Scheme,
	name string, namespace string, labels map[string]string, portName []string, portNumber []int32) *corev1.Service {
	ports := []corev1.ServicePort{}
	for i := range portName {
		port := corev1.ServicePort{
			Name:     portName[i],
			Port:     portNumber[i],
			Protocol: "TCP",
		}
		ports = append(ports, port)
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports:    ports,
			Selector: labels,
		},
	}

	// Set Workshop instance as the owner and controller
	err := ctrl.SetControllerReference(workshop, service, scheme)
	if err != nil {
		log.Error(err, " - Failed to set SetControllerReference for Service.")
	}
	return service
}

// NewCustomService creates a custom service
func NewCustomService(workshop *workshopv1.Workshop, scheme *runtime.Scheme,
	name string, namespace string, labels map[string]string, portName []string, portNumber []int32, targetPortNumber []intstr.IntOrString) *corev1.Service {
	ports := []corev1.ServicePort{}
	for i := range portName {
		port := corev1.ServicePort{
			Name:       portName[i],
			Port:       portNumber[i],
			TargetPort: targetPortNumber[i],
			Protocol:   "TCP",
		}
		ports = append(ports, port)
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: ports,
		},
	}

	// Set Workshop instance as the owner and controller
	err := ctrl.SetControllerReference(workshop, service, scheme)
	if err != nil {
		log.Error(err, " - Failed to set SetControllerReference for Custom Service - %s", name)
	}
	return service
}

// NewServiceWithTarget creates a  service with a specific target
func NewServiceWithTarget(workshop *workshopv1.Workshop, scheme *runtime.Scheme,
	name string, namespace string, labels map[string]string, portName []string, portNumber []int32, targetPortNumber []int32) *corev1.Service {
	ports := []corev1.ServicePort{}
	for i := range portName {
		port := corev1.ServicePort{
			Name: portName[i],
			Port: portNumber[i],
			TargetPort: intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: targetPortNumber[i],
			},
			Protocol: "TCP",
		}
		ports = append(ports, port)
	}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports:    ports,
			Selector: labels,
		},
	}

	// Set Workshop instance as the owner and controller
	err := ctrl.SetControllerReference(workshop, service, scheme)
	if err != nil {
		log.Error(err, " - Failed to set SetControllerReference for Service with specific target - %s", name)
	}
	return service
}
