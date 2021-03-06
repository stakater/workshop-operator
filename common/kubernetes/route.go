package kubernetes

import (
	routev1 "github.com/openshift/api/route/v1"
	"github.com/prometheus/common/log"
	workshopv1 "github.com/stakater/workshop-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)

// NewRoute creates an OpenShift Route
func NewRoute(workshop *workshopv1.Workshop, scheme *runtime.Scheme,
	name string, namespace string, labels map[string]string, serviceName string, port int32) *routev1.Route {

	targetPort := intstr.IntOrString{
		Type:   intstr.Int,
		IntVal: int32(port),
	}

	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: serviceName,
			},
			Port: &routev1.RoutePort{
				TargetPort: targetPort,
			},
		},
	}
	return route
}

// NewSecuredRoute creates an OpenShift Secured Route
func NewSecuredRoute(workshop *workshopv1.Workshop, scheme *runtime.Scheme,
	name string, namespace string, labels map[string]string, serviceName string, port int32) *routev1.Route {

	targetPort := intstr.IntOrString{
		Type:   intstr.Int,
		IntVal: int32(port),
	}

	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: serviceName,
			},
			Port: &routev1.RoutePort{
				TargetPort: targetPort,
			},
			TLS: &routev1.TLSConfig{
				Termination: routev1.TLSTerminationEdge,
			},
		},
	}

	// Set Workshop instance as the owner and controller
	err := ctrl.SetControllerReference(workshop, route, scheme)
	if err != nil {
		log.Error(err, " - Failed to set SetControllerReference for OpenShift Secured Route - %s", name)
	}
	return route
}
