package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	resourcev1 "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	myapigroupv1alpha1 "github.com/creiche/podinfo-operator/api/v1alpha1"
)

var _ = Describe("MyAppResource controller", func() {
	Context("MyAppResource controller test", func() {

		const MyAppResourceName = "test-myappresource"

		ctx := context.Background()

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      MyAppResourceName,
				Namespace: MyAppResourceName,
			},
		}

		typeMyAppResourceNamespaceName := types.NamespacedName{Name: MyAppResourceName, Namespace: MyAppResourceName}
		typePodinfoNamespaceName := types.NamespacedName{Name: podinfoName(MyAppResourceName), Namespace: MyAppResourceName}
		typeRedisNamespaceName := types.NamespacedName{Name: redisName(MyAppResourceName), Namespace: MyAppResourceName}

		BeforeEach(func() {
			By("Creating the Namespace to perform the tests")
			err := k8sClient.Create(ctx, namespace)
			Expect(err).To(Not(HaveOccurred()))
		})

		AfterEach(func() {
			By("Deleting the Namespace to perform the tests")
			_ = k8sClient.Delete(ctx, namespace)
		})

		It("should successfully reconcile a custom resource for MyAppResource", func() {
			By("Creating the custom resource for the Kind MyAppResource")
			myappresource := &myapigroupv1alpha1.MyAppResource{}
			err := k8sClient.Get(ctx, typeMyAppResourceNamespaceName, myappresource)
			if err != nil && errors.IsNotFound(err) {
				myappresource := &myapigroupv1alpha1.MyAppResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      MyAppResourceName,
						Namespace: namespace.Name,
					},
					Spec: myapigroupv1alpha1.MyAppResourceSpec{
						ReplicaCount: 2,
						Resources: myapigroupv1alpha1.MyAppResourceSpecResources{
							MemoryLimit: "64Mi",
							CpuRequest:  "100m",
						},
						Image: myapigroupv1alpha1.MyAppResourceSpecImage{
							Repository: "ghcr.io/stefanprodan/podinfo",
							Tag:        "latest",
						},
						Ui: myapigroupv1alpha1.MyAppResourceSpecUi{
							Color:   "#34577c",
							Message: "some string",
						},
						Redis: myapigroupv1alpha1.MyAppResourceSpecRedis{
							Enabled: true,
						},
					},
				}

				err = k8sClient.Create(ctx, myappresource)
				Expect(err).To(Not(HaveOccurred()))
			}

			By("Checking if the custom resource was successfully created")
			foundMyAppResource := &myapigroupv1alpha1.MyAppResource{}
			Eventually(func() error {
				return k8sClient.Get(ctx, typeMyAppResourceNamespaceName, foundMyAppResource)
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking if Podinfo Deployment was successfully created in the reconciliation")
			foundPodinfoDeployment := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, typePodinfoNamespaceName, foundPodinfoDeployment)
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking if Podinfo Service was successfully created in the reconciliation")
			foundPodinfoService := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(ctx, typePodinfoNamespaceName, foundPodinfoService)
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking if Redis StatefulSet was successfully created in the reconciliation")
			foundRedisStatefulSet := &appsv1.StatefulSet{}
			Eventually(func() error {
				return k8sClient.Get(ctx, typeRedisNamespaceName, foundRedisStatefulSet)
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking if Redis Service was successfully created in the reconciliation")
			foundRedisService := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(ctx, typeRedisNamespaceName, foundRedisService)
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking if Ownership is correct")
			expectedOwnerReference := metav1.OwnerReference{
				Kind:       "MyAppResource",
				APIVersion: "my.api.group/v1alpha1",
				UID:        foundMyAppResource.UID,
				Name:       foundMyAppResource.Name,
			}
			Expect(foundPodinfoDeployment.OwnerReferences).To(ContainElement(expectedOwnerReference))

			By("Checking if Env Vars are set properly")
			Expect(foundPodinfoDeployment.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "PODINFO_UI_COLOR", Value: foundMyAppResource.Spec.Ui.Color}))
			Expect(foundPodinfoDeployment.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "PODINFO_UI_MESSAGE", Value: foundMyAppResource.Spec.Ui.Message}))
			Expect(foundPodinfoDeployment.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "PODINFO_CACHE_SERVER", Value: "tcp://" + redisName(foundMyAppResource.Name) + ":6379"}))

			By("Checking if Resources are set on Podinfo")
			expectedResources := corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resourcev1.MustParse(foundMyAppResource.Spec.Resources.CpuRequest),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resourcev1.MustParse(foundMyAppResource.Spec.Resources.MemoryLimit),
				},
			}

			Expect(foundPodinfoDeployment.Spec.Template.Spec.Containers[0].Resources).To(Equal(expectedResources))

			By("Checking if Podinfo Deployment updates when updated")
			foundMyAppResource.Spec.Ui.Color = "#123456"
			foundMyAppResource.Spec.Ui.Message = "This is a new message"
			foundMyAppResource.Spec.Resources = myapigroupv1alpha1.MyAppResourceSpecResources{
				MemoryLimit: "128Mi",
				CpuRequest:  "200m",
			}
			Expect(k8sClient.Update(ctx, foundMyAppResource)).To(Succeed())
			Expect(k8sClient.Get(ctx, typeMyAppResourceNamespaceName, foundMyAppResource)).To(Succeed())

			expectedResources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resourcev1.MustParse(foundMyAppResource.Spec.Resources.CpuRequest),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resourcev1.MustParse(foundMyAppResource.Spec.Resources.MemoryLimit),
				},
			}

			Eventually(func() []corev1.EnvVar {
				k8sClient.Get(ctx, typePodinfoNamespaceName, foundPodinfoDeployment)
				return foundPodinfoDeployment.Spec.Template.Spec.Containers[0].Env
			}, time.Minute, time.Second).Should(ContainElement(corev1.EnvVar{Name: "PODINFO_UI_COLOR", Value: foundMyAppResource.Spec.Ui.Color}))
			Expect(foundPodinfoDeployment.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "PODINFO_UI_MESSAGE", Value: foundMyAppResource.Spec.Ui.Message}))
			Expect(foundPodinfoDeployment.Spec.Template.Spec.Containers[0].Resources).To(Equal(expectedResources))

			By("Checking if disabling Redis removes it")
			foundMyAppResource.Spec.Redis.Enabled = false
			Expect(k8sClient.Update(ctx, foundMyAppResource)).To(Succeed())
			Expect(k8sClient.Get(ctx, typeMyAppResourceNamespaceName, foundMyAppResource)).To(Succeed())

			Eventually(func() bool {
				deployment := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, typeRedisNamespaceName, deployment)
				return errors.IsNotFound(err)
			}, time.Minute, time.Second).Should(BeTrue())

			Eventually(func() bool {
				svc := &corev1.Service{}
				err := k8sClient.Get(ctx, typeRedisNamespaceName, svc)
				return errors.IsNotFound(err)
			}, time.Minute, time.Second).Should(BeTrue())

			Eventually(func() []corev1.EnvVar {
				k8sClient.Get(ctx, typePodinfoNamespaceName, foundPodinfoDeployment)
				return foundPodinfoDeployment.Spec.Template.Spec.Containers[0].Env
			}, time.Minute, time.Second).Should(Not(ContainElement(corev1.EnvVar{Name: "PODINFO_CACHE_SERVER", Value: "tcp://" + redisName(foundMyAppResource.Name) + ":6379"})))

		})
	})
})
