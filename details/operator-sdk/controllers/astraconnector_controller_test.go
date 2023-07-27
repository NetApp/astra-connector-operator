/*
 * Copyright (c) 2023. NetApp, Inc. All Rights Reserved.
 */

package controllers

import (
	"context"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
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
			err := k8sClient.Create(ctx, namespace)
			Expect(err).To(Not(HaveOccurred()))
		})

		AfterEach(func() {
			By("Deleting the Namespace to perform the tests")
			_ = k8sClient.Delete(ctx, namespace)
		})

		It("should successfully reconcile a custom resource for astraconnector", func() {
			By("creating the custom resource for the astraconnector")
			astraconnector := &v1.AstraConnector{}
			err := k8sClient.Get(ctx, typeNamespaceName, astraconnector)

			if err != nil && errors.IsNotFound(err) {
				// Mocking CR
				connector := &v1.AstraConnector{
					ObjectMeta: metav1.ObjectMeta{
						Name:      connectorName,
						Namespace: namespace.Name,
					},
					Spec: v1.AstraConnectorSpec{
						ImageRegistry: v1.ImageRegistry{
							Name:   "test-registry",
							Secret: "test-secret",
						},

						AstraConnect: v1.AstraConnect{
							Image:    "test-image",
							Replicas: 3,
						},
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

			_, err = controller.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespaceName,
			})
			Expect(err).To(Not(HaveOccurred()))

			By("Checking if Deployment was successfully created in the reconciliation")
			Eventually(func() error {
				found := &appsv1.Deployment{}
				return k8sClient.Get(ctx, typeNamespaceName, found)
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking the latest Status Condition added to the AstraConnector instance")
			Eventually(func() error {
				// TODO
				return nil
			}, time.Minute, time.Second).Should(Succeed())
		})
	})
})
