package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

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

		typeNamespaceName := types.NamespacedName{Name: MyAppResourceName, Namespace: MyAppResourceName}

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
			err := k8sClient.Get(ctx, typeNamespaceName, myappresource)
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
				return k8sClient.Get(ctx, typeNamespaceName, foundMyAppResource)
			}, time.Minute, time.Second).Should(Succeed())

			By("Reconciling the custom resource created")
			myappresourceReconciler := &MyAppResourceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err = myappresourceReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespaceName,
			})
			Expect(err).To(Not(HaveOccurred()))

			By("Checking if Deployment was successfully created in the reconciliation")
			foundPodinfoDeployment := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespaceName, foundPodinfoDeployment)
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking if Env Vars are set properly")
			Expect(foundPodinfoDeployment.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "PODINFO_UI_COLOR", Value: foundMyAppResource.Spec.Ui.Color}))
			Expect(foundPodinfoDeployment.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "PODINFO_UI_MESSAGE", Value: foundMyAppResource.Spec.Ui.Message}))

		})
	})
})
