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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	myapigroupv1alpha1 "github.com/creiche/podinfo-operator/api/v1alpha1"
)

// MyAppResourceReconciler reconciles a MyAppResource object
type MyAppResourceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=my.api.group.api.group,resources=myappresources,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=my.api.group.api.group,resources=myappresources/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=my.api.group.api.group,resources=myappresources/finalizers,verbs=update

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

	err = r.EnsureDeployment(ctx, *deploymentForPodinfo(myappresource))
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
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

	if found.Spec.Replicas != deployment.Spec.Replicas {
		found.Spec.Replicas = deployment.Spec.Replicas
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

func deploymentForPodinfo(myappresource *myapigroupv1alpha1.MyAppResource) *appsv1.Deployment {
	lbls := labelsForMyAppResource(myappresource)

	return &appsv1.Deployment{
		ObjectMeta: v1.ObjectMeta{
			Name:      myappresource.Name,
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
					}},
				},
			},
		},
	}
}

func labelsForMyAppResource(myappresource *myapigroupv1alpha1.MyAppResource) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "MyAppResource",
		"app.kubernetes.io/instance":   myappresource.Name,
		"app.kubernetes.io/version":    myappresource.Spec.Image.Tag,
		"app.kubernetes.io/part-of":    "myappresource",
		"app.kubernetes.io/created-by": "podinfo-operator",
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *MyAppResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&myapigroupv1alpha1.MyAppResource{}).
		Complete(r)
}
