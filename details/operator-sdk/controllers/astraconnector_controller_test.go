/*
 * Copyright (c) 2023. NetApp, Inc. All Rights Reserved.
 */

package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
			err := k8sClient.Get(ctx, types.NamespacedName{Name: connectorName, Namespace: namespace.Name}, astraConnector)
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

func TestDeleteNeptuneResources(t *testing.T) {
	mockScheme := runtime.NewScheme()
	assert.Nil(t, apiextensionsv1.AddToScheme(mockScheme))

	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "applications.astra.netapp.io",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "astra.netapp.io",
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name: "v1",
				},
			},
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural: "applications",
			},
		},
	}

	// Define some test objects
	testObjects := []runtime.Object{
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "astra.netapp.io/v1",
				"kind":       "Application", // This matches the kind of the CRD
				"metadata": map[string]interface{}{
					"name":      "test-application",
					"namespace": "test-namespace",
				},
			},
		},
	}

	fakeCtrlRuntime := fake.NewClientBuilder().WithScheme(mockScheme).WithObjects(crd).Build()

	// Create a mock dynamic client
	mockDynamicClient := &mockDynamicInterface{
		resources: make(map[string]*unstructured.Unstructured), // Initialize the resources map
	}

	// Add the test objects to the mock dynamic client
	for _, obj := range testObjects {
		unstrObj, ok := obj.(*unstructured.Unstructured)
		if !ok {
			t.Fatalf("test object is not *unstructured.Unstructured: %v", obj)
		}
		mockDynamicClient.resources[unstrObj.GetName()] = unstrObj
	}

	// Create a controller with the fake clients
	controller := &AstraConnectorController{
		Client:        fakeCtrlRuntime,
		DynamicClient: mockDynamicClient,
	}

	// Call the function
	controller.deleteNeptuneResources(context.Background(), "test-namespace")

	// Check that the function deleted the resources
	list, err := mockDynamicClient.Namespace("test-namespace").List(context.Background(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Empty(t, list.Items)
}

type mockDynamicInterface struct {
	// Store resources in a map keyed by name
	resources map[string]*unstructured.Unstructured
}

func (m *mockDynamicInterface) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return m
}

func (m *mockDynamicInterface) Namespace(namespace string) dynamic.ResourceInterface {
	return m
}

func (m *mockDynamicInterface) Delete(ctx context.Context, name string, opts metav1.DeleteOptions, subresources ...string) error {
	if _, exists := m.resources[name]; !exists {
		return fmt.Errorf("resource not found: %s", name)
	}
	delete(m.resources, name)
	return nil
}

func (m *mockDynamicInterface) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	list := &unstructured.UnstructuredList{}
	for _, resource := range m.resources {
		list.Items = append(list.Items, *resource)
	}
	return list, nil
}

// Implement the other methods of the dynamic.ResourceInterface here...
// You can leave them empty or add minimal implementations if you don't need them for your tests
func (m *mockDynamicInterface) Get(ctx context.Context, name string, opts metav1.GetOptions, subR ...string) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (m *mockDynamicInterface) Create(ctx context.Context, obj *unstructured.Unstructured, opts metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	m.resources[obj.GetName()] = obj
	return obj, nil
}

func (m *mockDynamicInterface) Update(ctx context.Context, obj *unstructured.Unstructured, opts metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (m *mockDynamicInterface) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, opts metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (m *mockDynamicInterface) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return nil, nil
}

func (m *mockDynamicInterface) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (m *mockDynamicInterface) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	m.resources = make(map[string]*unstructured.Unstructured)
	return nil
}

func (m *mockDynamicInterface) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, applyOpts metav1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (m *mockDynamicInterface) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, applyOpts metav1.ApplyOptions) (*unstructured.Unstructured, error) {
	return nil, nil
}
