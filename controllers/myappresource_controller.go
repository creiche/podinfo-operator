/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	resourcev1 "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	myapigroupv1alpha1 "github.com/creiche/podinfo-operator/api/v1alpha1"
)

// MyAppResourceReconciler reconciles a MyAppResource object
type MyAppResourceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=my.api.group,resources=myappresources,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=my.api.group,resources=myappresources/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=my.api.group,resources=myappresources/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MyAppResource object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *MyAppResourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	myappresource := &myapigroupv1alpha1.MyAppResource{}
	err := r.Get(ctx, req.NamespacedName, myappresource)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("myappresource resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get myappresource")
		return ctrl.Result{}, err
	}

	if myappresource.Spec.Redis.Enabled == true {
		err = r.EnsureStatefulSet(ctx, *r.statefulsetForRedis(myappresource))
		if err != nil {
			return ctrl.Result{}, err
		}

		err = r.EnsureService(ctx, *r.serviceForRedis(myappresource))
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		// Check if the objects exist, if so we should remove them
		redisStatefulSet := r.statefulsetForRedis(myappresource)
		redisService := r.serviceForRedis(myappresource)
		err := r.Get(ctx, types.NamespacedName{Namespace: redisStatefulSet.Namespace, Name: redisStatefulSet.Name}, redisStatefulSet)
		if err == nil {
			r.Client.Delete(ctx, r.statefulsetForRedis(myappresource))
		}
		err = r.Get(ctx, types.NamespacedName{Namespace: redisService.Namespace, Name: redisService.Name}, redisService)
		if err == nil {
			r.Client.Delete(ctx, r.serviceForRedis(myappresource))
		}
	}

	// We don't want to create the app until Redis is ready if it's enabled
	if myappresource.Spec.Redis.Enabled == true && r.IsStatefulSetReady(ctx, myappresource.Namespace, redisName(myappresource.Name)) {
		err = r.EnsureDeployment(ctx, *r.deploymentForPodinfo(myappresource))
		if err != nil {
			return ctrl.Result{}, err
		}

		err = r.EnsureService(ctx, *r.serviceForPodinfo(myappresource))
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// If Redis is not enabled, we don't care if it's ready
	if myappresource.Spec.Redis.Enabled == false {
		err = r.EnsureDeployment(ctx, *r.deploymentForPodinfo(myappresource))
		if err != nil {
			return ctrl.Result{}, err
		}

		err = r.EnsureService(ctx, *r.serviceForPodinfo(myappresource))
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	err = r.ReconcileStatus(ctx, myappresource)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *MyAppResourceReconciler) ReconcileStatus(ctx context.Context, myappresource *myapigroupv1alpha1.MyAppResource) (err error) {
	log := log.FromContext(ctx)
	changed := false

	// Get latest version of MyAppResource since we will be updating it
	err = r.Get(ctx, types.NamespacedName{Namespace: myappresource.Namespace, Name: myappresource.Name}, myappresource)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("myappresource resource not found. Ignoring since object must be deleted")
			return nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get myappresource")
		return err
	}

	// Check if Redis is Ready
	if myappresource.Spec.Redis.Enabled {
		if r.IsStatefulSetReady(ctx, myappresource.Namespace, redisName(myappresource.Name)) {
			redisReadyCondition := &metav1.Condition{
				Type:    "RedisReady",
				Status:  metav1.ConditionTrue,
				Reason:  "RedisStatefulSetReady",
				Message: "Redis StatefulSet's Replicas are ready",
			}
			if !meta.IsStatusConditionPresentAndEqual(myappresource.Status.Conditions, redisReadyCondition.Type, redisReadyCondition.Status) {
				meta.SetStatusCondition(&myappresource.Status.Conditions, *redisReadyCondition)
				changed = true
			}
		} else {
			redisReadyCondition := &metav1.Condition{
				Type:    "RedisReady",
				Status:  metav1.ConditionFalse,
				Reason:  "RedisStatefulSetReady",
				Message: "Redis StatefulSet's Replicas are not ready",
			}
			if !meta.IsStatusConditionPresentAndEqual(myappresource.Status.Conditions, redisReadyCondition.Type, redisReadyCondition.Status) {
				meta.SetStatusCondition(&myappresource.Status.Conditions, *redisReadyCondition)
				changed = true
			}
		}
	} else {
		// If Redis is not enabled, make sure the condition does not exist
		if meta.IsStatusConditionPresentAndEqual(myappresource.Status.Conditions, "RedisReady", metav1.ConditionFalse) || meta.IsStatusConditionPresentAndEqual(myappresource.Status.Conditions, "RedisReady", metav1.ConditionTrue) {
			meta.RemoveStatusCondition(&myappresource.Status.Conditions, "RedisReady")
			changed = true
		}
	}

	if r.IsDeploymentReady(ctx, myappresource.Namespace, podinfoName(myappresource.Name)) {
		podinfoReadyCondition := &metav1.Condition{
			Type:    "PodinfoReady",
			Status:  metav1.ConditionTrue,
			Reason:  "PodinfoDeploymentReady",
			Message: "Podinfo's Deployment's Replicas are ready",
		}
		if !meta.IsStatusConditionPresentAndEqual(myappresource.Status.Conditions, podinfoReadyCondition.Type, podinfoReadyCondition.Status) {
			meta.SetStatusCondition(&myappresource.Status.Conditions, *podinfoReadyCondition)
			changed = true
		}
	} else {
		podinfoReadyCondition := &metav1.Condition{
			Type:    "PodinfoReady",
			Status:  metav1.ConditionFalse,
			Reason:  "PodinfoDeploymentReady",
			Message: "Podinfo's Deployment's Replicas are not ready",
		}
		if !meta.IsStatusConditionPresentAndEqual(myappresource.Status.Conditions, podinfoReadyCondition.Type, podinfoReadyCondition.Status) {
			meta.SetStatusCondition(&myappresource.Status.Conditions, *podinfoReadyCondition)
			changed = true
		}
	}

	if changed == true {
		err = r.Status().Update(ctx, myappresource)
		if err != nil {
			log.Error(err, "Failed to update status")
			return
		}
	}
	return
}

func (r *MyAppResourceReconciler) IsStatefulSetReady(ctx context.Context, namespace string, name string) bool {
	statefulset := &appsv1.StatefulSet{}
	err := r.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, statefulset)
	if err != nil {
		return false
	}

	if statefulset.Status.ReadyReplicas == *statefulset.Spec.Replicas {
		return true
	}

	return false
}

func (r *MyAppResourceReconciler) IsDeploymentReady(ctx context.Context, namespace string, name string) bool {
	deployment := &appsv1.Deployment{}
	err := r.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, deployment)
	if err != nil {
		return false
	}

	if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas {
		return true
	}

	return false
}

func (r *MyAppResourceReconciler) EnsureDeployment(ctx context.Context, deployment appsv1.Deployment) error {
	found := &appsv1.Deployment{}

	err := r.Get(ctx, types.NamespacedName{Namespace: deployment.Namespace, Name: deployment.Name}, found)
	if err != nil && apierrors.IsNotFound(err) {
		if err = r.Create(ctx, &deployment); err != nil {
			log.Log.Error(err, "Failed to create deployment", "deployment.Namespace", deployment.Namespace, "deployment.Name", deployment.Name)
			return err
		}
		return nil
	}

	changed := false

	if found.Spec.Template.Spec.Containers[0].Image != deployment.Spec.Template.Spec.Containers[0].Image {
		found.Spec.Template.Spec.Containers[0].Image = deployment.Spec.Template.Spec.Containers[0].Image
		changed = true
	}

	if !reflect.DeepEqual(found.Spec.Template.Spec.Containers[0].Env, deployment.Spec.Template.Spec.Containers[0].Env) {
		found.Spec.Template.Spec.Containers[0].Env = deployment.Spec.Template.Spec.Containers[0].Env
		changed = true
	}

	if found.Spec.Replicas != deployment.Spec.Replicas {
		found.Spec.Replicas = deployment.Spec.Replicas
		changed = true
	}

	if !reflect.DeepEqual(found.Spec.Template.Spec.Containers[0].Resources, deployment.Spec.Template.Spec.Containers[0].Resources) {
		found.Spec.Template.Spec.Containers[0].Resources = deployment.Spec.Template.Spec.Containers[0].Resources
		changed = true
	}

	if changed == true {
		if err = r.Update(ctx, found); err != nil {
			log.Log.Error(err, "Failed to update deployment", "deployment.Namespace", deployment.Namespace, "deployment.Name", deployment.Name)
			return err
		}
	}

	return nil
}

func (r *MyAppResourceReconciler) EnsureStatefulSet(ctx context.Context, statefulset appsv1.StatefulSet) error {
	found := &appsv1.StatefulSet{}

	err := r.Get(ctx, types.NamespacedName{Namespace: statefulset.Namespace, Name: statefulset.Name}, found)
	if err != nil && apierrors.IsNotFound(err) {
		if err = r.Create(ctx, &statefulset); err != nil {
			log.Log.Error(err, "Failed to create statefulset", "statefulset.Namespace", statefulset.Namespace, "statefulset.Name", statefulset.Name)
			return err
		}
		return nil
	}

	changed := false

	if found.Spec.Template.Spec.Containers[0].Image != statefulset.Spec.Template.Spec.Containers[0].Image {
		found.Spec.Template.Spec.Containers[0].Image = statefulset.Spec.Template.Spec.Containers[0].Image
		changed = true
	}

	if !reflect.DeepEqual(found.Spec.Template.Spec.Containers[0].Env, statefulset.Spec.Template.Spec.Containers[0].Env) {
		found.Spec.Template.Spec.Containers[0].Env = statefulset.Spec.Template.Spec.Containers[0].Env
		changed = true
	}

	if found.Spec.Replicas != statefulset.Spec.Replicas {
		found.Spec.Replicas = statefulset.Spec.Replicas
		changed = true
	}

	if !reflect.DeepEqual(found.Spec.Template.Spec.Containers[0].Resources, statefulset.Spec.Template.Spec.Containers[0].Resources) {
		found.Spec.Template.Spec.Containers[0].Resources = statefulset.Spec.Template.Spec.Containers[0].Resources
		changed = true
	}

	if changed == true {
		if err = r.Update(ctx, found); err != nil {
			log.Log.Error(err, "Failed to update statefulset", "statefulset.Namespace", statefulset.Namespace, "statefulset.Name", statefulset.Name)
			return err
		}
	}

	return nil
}

func (r *MyAppResourceReconciler) EnsureService(ctx context.Context, service corev1.Service) error {
	found := &corev1.Service{}

	err := r.Get(ctx, types.NamespacedName{Namespace: service.Namespace, Name: service.Name}, found)
	if err != nil && apierrors.IsNotFound(err) {
		if err = r.Create(ctx, &service); err != nil {
			log.Log.Error(err, "Failed to create service", "service.Namespace", service.Namespace, "service.Name", service.Name)
			return err
		}
		return nil
	}

	changed := false

	if !reflect.DeepEqual(found.Spec.Selector, service.Spec.Selector) {
		found.Spec.Selector = service.Spec.Selector
		changed = true
	}

	if changed == true {
		if err = r.Update(ctx, found); err != nil {
			log.Log.Error(err, "Failed to update service", "service.Namespace", service.Namespace, "service.Name", service.Name)
			return err
		}
	}

	return nil
}

func (r *MyAppResourceReconciler) serviceForPodinfo(myappresource *myapigroupv1alpha1.MyAppResource) *corev1.Service {
	lbls := labelsForPodInfo(myappresource)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podinfoName(myappresource.Name),
			Namespace: myappresource.Namespace,
			Labels:    lbls,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       podinfoName(myappresource.Name),
					Port:       80,
					TargetPort: intstr.FromString("podinfo"),
				},
			},
			Selector: lbls,
			Type:     corev1.ServiceTypeClusterIP,
		},
	}

	controllerutil.SetControllerReference(myappresource, service, r.Scheme)

	return service
}

func (r *MyAppResourceReconciler) serviceForRedis(myappresource *myapigroupv1alpha1.MyAppResource) *corev1.Service {
	lbls := labelsForRedis(myappresource)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      redisName(myappresource.Name),
			Namespace: myappresource.Namespace,
			Labels:    lbls,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       podinfoName(myappresource.Name),
					Port:       6379,
					TargetPort: intstr.FromString("redis"),
				},
			},
			Selector: lbls,
			Type:     corev1.ServiceTypeClusterIP,
		},
	}

	controllerutil.SetControllerReference(myappresource, service, r.Scheme)

	return service
}

func (r MyAppResourceReconciler) deploymentForPodinfo(myappresource *myapigroupv1alpha1.MyAppResource) *appsv1.Deployment {
	lbls := labelsForPodInfo(myappresource)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podinfoName(myappresource.Name),
			Namespace: myappresource.Namespace,
			Labels:    lbls,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &myappresource.Spec.ReplicaCount,
			Selector: &metav1.LabelSelector{
				MatchLabels: lbls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: lbls,
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &[]bool{true}[0],
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{{
						Image: myappresource.Spec.Image.Repository + ":" + myappresource.Spec.Image.Tag,
						Name:  "podinfo",
						Env: []corev1.EnvVar{
							{
								Name:  "PODINFO_UI_COLOR",
								Value: myappresource.Spec.Ui.Color,
							},
							{
								Name:  "PODINFO_UI_MESSAGE",
								Value: myappresource.Spec.Ui.Message,
							},
						},
						ImagePullPolicy: corev1.PullIfNotPresent,
						SecurityContext: &corev1.SecurityContext{
							RunAsNonRoot:             &[]bool{true}[0],
							RunAsUser:                &[]int64{1001}[0],
							AllowPrivilegeEscalation: &[]bool{false}[0],
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{
									"ALL",
								},
							},
						},
						Ports: []corev1.ContainerPort{{
							ContainerPort: 9898,
							Name:          "podinfo",
						}},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resourcev1.MustParse(myappresource.Spec.Resources.CpuRequest),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: resourcev1.MustParse(myappresource.Spec.Resources.MemoryLimit),
							},
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/healthz",
									Port: intstr.FromString("podinfo"),
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/readyz",
									Port: intstr.FromString("podinfo"),
								},
							},
						},
					}},
				},
			},
		},
	}

	if myappresource.Spec.Redis.Enabled == true {
		deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  "PODINFO_CACHE_SERVER",
			Value: "tcp://" + redisName(myappresource.Name) + ":6379",
		})
	}

	controllerutil.SetControllerReference(myappresource, deployment, r.Scheme)

	return deployment
}

func (r MyAppResourceReconciler) statefulsetForRedis(myappresource *myapigroupv1alpha1.MyAppResource) *appsv1.StatefulSet {
	lbls := labelsForRedis(myappresource)

	var replicas int32
	replicas = 1

	deployment := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      redisName(myappresource.Name),
			Namespace: myappresource.Namespace,
			Labels:    lbls,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: lbls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: lbls,
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &[]bool{true}[0],
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{{
						Image:           "redis:" + myappresource.Spec.Redis.Tag,
						Name:            "redis",
						ImagePullPolicy: corev1.PullIfNotPresent,
						SecurityContext: &corev1.SecurityContext{
							RunAsNonRoot:             &[]bool{true}[0],
							RunAsUser:                &[]int64{999}[0],
							AllowPrivilegeEscalation: &[]bool{false}[0],
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{
									"ALL",
								},
							},
						},
						Ports: []corev1.ContainerPort{{
							ContainerPort: 6379,
							Name:          "redis",
						}},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								Exec: &corev1.ExecAction{
									Command: []string{
										"redis-cli",
										"ping",
									},
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								Exec: &corev1.ExecAction{
									Command: []string{
										"redis-cli",
										"ping",
									},
								},
							},
						},
					}},
					Volumes: []corev1.Volume{
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	controllerutil.SetControllerReference(myappresource, deployment, r.Scheme)

	return deployment
}

func labelsForPodInfo(myappresource *myapigroupv1alpha1.MyAppResource) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       podinfoName(myappresource.Name),
		"app.kubernetes.io/instance":   myappresource.Name,
		"app.kubernetes.io/version":    myappresource.Spec.Image.Tag,
		"app.kubernetes.io/part-of":    "myappresource",
		"app.kubernetes.io/created-by": "podinfo-operator",
	}
}

func labelsForRedis(myappresource *myapigroupv1alpha1.MyAppResource) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       redisName(myappresource.Name),
		"app.kubernetes.io/instance":   myappresource.Name,
		"app.kubernetes.io/version":    myappresource.Spec.Redis.Tag,
		"app.kubernetes.io/part-of":    "myappresource",
		"app.kubernetes.io/created-by": "podinfo-operator",
	}
}

func podinfoName(name string) string {
	return name + "-podinfo"
}

func redisName(name string) string {
	return name + "-redis"
}

// SetupWithManager sets up the controller with the Manager.
func (r *MyAppResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&myapigroupv1alpha1.MyAppResource{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
