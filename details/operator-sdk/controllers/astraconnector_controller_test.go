/*
 * Copyright (c) 2023. NetApp, Inc. All Rights Reserved.
 */

package controllers

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
)

var _ = Describe("Astraconnector controller", func() {
	Context("Astraconnector controller test", func() {
		const connectorName = "astra-connector-operator"

		ctx := context.Background()

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      connectorName,
				Namespace: connectorName,
			},
		}

		typeNamespaceName := types.NamespacedName{Name: connectorName, Namespace: connectorName}

		BeforeEach(func() {
			By("Creating the Namespace to perform the tests")
			ns := &corev1.Namespace{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: connectorName}, ns)
			if err == nil {
				// Namespace exists, delete it
				Expect(k8sClient.Delete(ctx, ns)).To(Succeed())
				// Wait for deletion to complete
				Eventually(func() error {
					return k8sClient.Get(ctx, types.NamespacedName{Name: connectorName}, ns)
				}, time.Minute, time.Second).ShouldNot(Succeed())
			}
			// Now create the namespace
			err = k8sClient.Create(ctx, namespace)
			Expect(err).To(Not(HaveOccurred()))

			// Wait for the namespace to be created
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{Name: namespace.Name}, &corev1.Namespace{})
			}, time.Minute, time.Second).Should(Succeed())
		})

		AfterEach(func() {
			By("Removing the finalizer from the AstraConnector custom resource")
			astraConnector := &v1.AstraConnector{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: connectorName, Namespace: namespace.Name},
				astraConnector)
			if err == nil {
				astraConnector.Finalizers = nil
				Expect(k8sClient.Update(ctx, astraConnector)).To(Succeed())
			}

			By("Deleting the AstraConnector custom resource")
			err = k8sClient.Delete(ctx, astraConnector)
			if err != nil && !errors.IsNotFound(err) {
				Expect(err).To(Not(HaveOccurred()))
			}

			By("Deleting the Namespace")
			Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())
		})

		It("should successfully reconcile a custom resource for astraconnector", func() {
			By("creating the custom resource for the astraconnector")
			astraConnector := &v1.AstraConnector{}
			err := k8sClient.Get(ctx, typeNamespaceName, astraConnector)

			if err != nil && errors.IsNotFound(err) {
				// Mocking CR
				connector := &v1.AstraConnector{
					ObjectMeta: metav1.ObjectMeta{
						Name:      connectorName,
						Namespace: namespace.Name,
					},
					Spec: v1.AstraConnectorSpec{
						Astra: v1.Astra{
							ClusterName: "managed-cluster",
						},
						AutoSupport: v1.AutoSupport{
							Enrolled: false,
						},
						ImageRegistry: v1.ImageRegistry{
							Name:   "test-registry",
							Secret: "test-secret",
						},
						AstraConnect: v1.AstraConnect{
							Image:    "test-image",
							Replicas: 3,
						},
						SkipPreCheck: true,
					},
				}

				err = k8sClient.Create(ctx, connector)
				Expect(err).To(Not(HaveOccurred()))
			}

			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &v1.AstraConnector{}
				return k8sClient.Get(ctx, typeNamespaceName, found)
			}, time.Minute, time.Second).Should(Succeed())

			By("Reconciling the custom resource created")
			controller := &AstraConnectorController{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := controller.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespaceName,
			})
			Expect(err).To(Not(HaveOccurred()))

			By("Checking if natssync-client deployment was successfully created in the reconciliation")
			Eventually(func() error {
				found := &appsv1.Deployment{}
				natsSyncName := types.NamespacedName{Name: "natssync-client", Namespace: connectorName}
				return k8sClient.Get(ctx, natsSyncName, found)
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking if nats statefulset was successfully created in the reconciliation")
			Eventually(func() error {
				found := &appsv1.StatefulSet{}
				natsName := types.NamespacedName{Name: "nats", Namespace: connectorName}
				return k8sClient.Get(ctx, natsName, found)
			}, time.Minute, time.Second).Should(Succeed())

			By("checking the Reconcile result")
			Expect(result.Requeue).To(BeFalse())

			By("checking the AstraConnector status")
			ac := &v1.AstraConnector{}
			err = k8sClient.Get(context.Background(), typeNamespaceName, ac)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
