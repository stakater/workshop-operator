```bash

INFO[0199] OpenShift Console URL https://console-openshift-console.apps.workshop.6hf8sw9d.kubeapp.cloud  source="workshop_controller.go:102"
INFO[0199] Apps Hostname Suffix apps.workshop.6hf8sw9d.kubeapp.cloud  source="workshop_controller.go:107"
2021-12-28T16:32:40.883+0530	INFO	controllers.Workshop	Adding Finalizer for the Workshop	{"workshop": "workshop-infra/cloud-native-workshop"}
2021-12-28T16:32:41.198+0530	ERROR	controllers.Workshop	Failed to update Workshop with finalizer	{"workshop": "workshop-infra/cloud-native-workshop", "error": "Operation cannot be fulfilled on workshops.workshop.stakater.com \"cloud-native-workshop\": the object has been modified; please apply your changes to the latest version and try again"}
github.com/go-logr/zapr.(*zapLogger).Error
	/home/maaz/ws/public/stakater/workshop-operator/vendor/github.com/go-logr/zapr/zapr.go:132
github.com/stakater/workshop-operator/controllers.(*WorkshopReconciler).addFinalizer
	/home/maaz/ws/public/stakater/workshop-operator/controllers/finalizer.go:27
github.com/stakater/workshop-operator/controllers.(*WorkshopReconciler).Reconcile
	/home/maaz/ws/public/stakater/workshop-operator/controllers/workshop_controller.go:142
sigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).reconcileHandler
	/home/maaz/ws/public/stakater/workshop-operator/vendor/sigs.k8s.io/controller-runtime/pkg/internal/controller/controller.go:244
sigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).processNextWorkItem
	/home/maaz/ws/public/stakater/workshop-operator/vendor/sigs.k8s.io/controller-runtime/pkg/internal/controller/controller.go:218
sigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).worker
	/home/maaz/ws/public/stakater/workshop-operator/vendor/sigs.k8s.io/controller-runtime/pkg/internal/controller/controller.go:197
k8s.io/apimachinery/pkg/util/wait.BackoffUntil.func1
	/home/maaz/ws/public/stakater/workshop-operator/vendor/k8s.io/apimachinery/pkg/util/wait/wait.go:155
k8s.io/apimachinery/pkg/util/wait.BackoffUntil
	/home/maaz/ws/public/stakater/workshop-operator/vendor/k8s.io/apimachinery/pkg/util/wait/wait.go:156
k8s.io/apimachinery/pkg/util/wait.JitterUntil
	/home/maaz/ws/public/stakater/workshop-operator/vendor/k8s.io/apimachinery/pkg/util/wait/wait.go:133
k8s.io/apimachinery/pkg/util/wait.Until
	/home/maaz/ws/public/stakater/workshop-operator/vendor/k8s.io/apimachinery/pkg/util/wait/wait.go:90
2021-12-28T16:32:41.198+0530	ERROR	controller	Reconciler error	{"reconcilerGroup": "workshop.stakater.com", "reconcilerKind": "Workshop", "controller": "workshop", "name": "cloud-native-workshop", "namespace": "workshop-infra", "error": "Operation cannot be fulfilled on workshops.workshop.stakater.com \"cloud-native-workshop\": the object has been modified; please apply your changes to the latest version and try again"}
github.com/go-logr/zapr.(*zapLogger).Error
	/home/maaz/ws/public/stakater/workshop-operator/vendor/github.com/go-logr/zapr/zapr.go:132
sigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).reconcileHandler
	/home/maaz/ws/public/stakater/workshop-operator/vendor/sigs.k8s.io/controller-runtime/pkg/internal/controller/controller.go:246
sigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).processNextWorkItem
	/home/maaz/ws/public/stakater/workshop-operator/vendor/sigs.k8s.io/controller-runtime/pkg/internal/controller/controller.go:218
sigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).worker
	/home/maaz/ws/public/stakater/workshop-operator/vendor/sigs.k8s.io/controller-runtime/pkg/internal/controller/controller.go:197
k8s.io/apimachinery/pkg/util/wait.BackoffUntil.func1
	/home/maaz/ws/public/stakater/workshop-operator/vendor/k8s.io/apimachinery/pkg/util/wait/wait.go:155
k8s.io/apimachinery/pkg/util/wait.BackoffUntil
	/home/maaz/ws/public/stakater/workshop-operator/vendor/k8s.io/apimachinery/pkg/util/wait/wait.go:156
k8s.io/apimachinery/pkg/util/wait.JitterUntil
	/home/maaz/ws/public/stakater/workshop-operator/vendor/k8s.io/apimachinery/pkg/util/wait/wait.go:133
k8s.io/apimachinery/pkg/util/wait.Until
	/home/maaz/ws/public/stakater/workshop-operator/vendor/k8s.io/apimachinery/pkg/util/wait/wait.go:90
INFO[0201] OpenShift Console URL https://console-openshift-console.apps.workshop.6hf8sw9d.kubeapp.cloud  source="workshop_controller.go:102"
INFO[0201] Apps Hostname Suffix apps.workshop.6hf8sw9d.kubeapp.cloud  source="workshop_controller.go:107"
ERRO[0201] cluster-scoped resource must not have a namespace-scoped owner, owner's namespace workshop-infra - Failed to set SetControllerReference for OpenShift user - %s&User{ObjectMeta:{user1      0 0001-01-01 00:00:00 +0000 UTC <nil> <nil> map[] map[] [] []  []},FullName:user1,Identities:[],Groups:[],}  source="user.go:24"
2021-12-28T16:32:42.200+0530	ERROR	controller	Reconciler error	{"reconcilerGroup": "workshop.stakater.com", "reconcilerKind": "Workshop", "controller": "workshop", "name": "cloud-native-workshop", "namespace": "workshop-infra", "error": "no kind is registered for the type v1.User in scheme \"/home/maaz/ws/public/stakater/workshop-operator/main.go:48\""}
github.com/go-logr/zapr.(*zapLogger).Error
	/home/maaz/ws/public/stakater/workshop-operator/vendor/github.com/go-logr/zapr/zapr.go:132
sigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).reconcileHandler
	/home/maaz/ws/public/stakater/workshop-operator/vendor/sigs.k8s.io/controller-runtime/pkg/internal/controller/controller.go:246
sigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).processNextWorkItem
	/home/maaz/ws/public/stakater/workshop-operator/vendor/sigs.k8s.io/controller-runtime/pkg/internal/controller/controller.go:218
sigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).worker
	/home/maaz/ws/public/stakater/workshop-operator/vendor/sigs.k8s.io/controller-runtime/pkg/internal/controller/controller.go:197
k8s.io/apimachinery/pkg/util/wait.BackoffUntil.func1
	/home/maaz/ws/public/stakater/workshop-operator/vendor/k8s.io/apimachinery/pkg/util/wait/wait.go:155
k8s.io/apimachinery/pkg/util/wait.BackoffUntil
	/home/maaz/ws/public/stakater/workshop-operator/vendor/k8s.io/apimachinery/pkg/util/wait/wait.go:156
k8s.io/apimachinery/pkg/util/wait.JitterUntil
	/home/maaz/ws/public/stakater/workshop-operator/vendor/k8s.io/apimachinery/pkg/util/wait/wait.go:133
k8s.io/apimachinery/pkg/util/wait.Until
	/home/maaz/ws/public/stakater/workshop-operator/vendor/k8s.io/apimachinery/pkg/util/wait/wait.go:90
INFO[0202] OpenShift Console URL https://console-openshift-console.apps.workshop.6hf8sw9d.kubeapp.cloud  source="workshop_controller.go:102"
INFO[0202] Apps Hostname Suffix apps.workshop.6hf8sw9d.kubeapp.cloud  source="workshop_controller.go:107"
ERRO[0202] cluster-scoped resource must not have a namespace-scoped owner, owner's namespace workshop-infra - Failed to set SetControllerReference for OpenShift user - %s&User{ObjectMeta:{user1      0 0001-01-01 00:00:00 +0000 UTC <nil> <nil> map[] map[] [] []  []},FullName:user1,Identities:[],Groups:[],}  source="user.go:24"
2021-12-28T16:32:43.202+0530	ERROR	controller	Reconciler error	{"reconcilerGroup": "workshop.stakater.com", "reconcilerKind": "Workshop", "controller": "workshop", "name": "cloud-native-workshop", "namespace": "workshop-infra", "error": "no kind is registered for the type v1.User in scheme \"/home/maaz/ws/public/stakater/workshop-operator/main.go:48\""}
github.com/go-logr/zapr.(*zapLogger).Error
	/home/maaz/ws/public/stakater/workshop-operator/vendor/github.com/go-logr/zapr/zapr.go:132
sigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).reconcileHandler
	/home/maaz/ws/public/stakater/workshop-operator/vendor/sigs.k8s.io/controller-runtime/pkg/internal/controller/controller.go:246
sigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).processNextWorkItem
	/home/maaz/ws/public/stakater/workshop-operator/vendor/sigs.k8s.io/controller-runtime/pkg/internal/controller/controller.go:218
sigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).worker
	/home/maaz/ws/public/stakater/workshop-operator/vendor/sigs.k8s.io/controller-runtime/pkg/internal/controller/controller.go:197
k8s.io/apimachinery/pkg/util/wait.BackoffUntil.func1
	/home/maaz/ws/public/stakater/workshop-operator/vendor/k8s.io/apimachinery/pkg/util/wait/wait.go:155
k8s.io/apimachinery/pkg/util/wait.BackoffUntil
	/home/maaz/ws/public/stakater/workshop-operator/vendor/k8s.io/apimachinery/pkg/util/wait/wait.go:156
k8s.io/apimachinery/pkg/util/wait.JitterUntil
	/home/maaz/ws/public/stakater/workshop-operator/vendor/k8s.io/apimachinery/pkg/util/wait/wait.go:133
k8s.io/apimachinery/pkg/util/wait.Until
	/home/maaz/ws/public/stakater/workshop-operator/vendor/k8s.io/apimachinery/pkg/util/wait/wait.go:90

``` 

```bash
NFO[0005] OpenShift Console URL https://console-openshift-console.apps.workshop.6hf8sw9d.kubeapp.cloud  source="workshop_controller.go:102"
INFO[0005] Apps Hostname Suffix apps.workshop.6hf8sw9d.kubeapp.cloud  source="workshop_controller.go:107"
ERRO[0005] cluster-scoped resource must not have a namespace-scoped owner, owner's namespace workshop-infra - Failed to set SetControllerReference for OpenShift user - %s&User{ObjectMeta:{user1      0 0001-01-01 00:00:00 +0000 UTC <nil> <nil> map[] map[] [] []  []},FullName:user1,Identities:[],Groups:[],}  source="user.go:24"
INFO[0005] Created user1 user                            source="user_reconciler.go:47"
ERRO[0005] cluster-scoped resource must not have a namespace-scoped owner, owner's namespace workshop-infra - Failed to set SetControllerReference for OpenShift user - %s&User{ObjectMeta:{user2      0 0001-01-01 00:00:00 +0000 UTC <nil> <nil> map[] map[] [] []  []},FullName:user2,Identities:[],Groups:[],}  source="user.go:24"

```