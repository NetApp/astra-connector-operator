package controllers

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

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
	err := controller.deleteNeptuneResources(context.Background(), "test-namespace")
	assert.NoError(t, err)

	// Check that the function deleted the resources
	list, err := mockDynamicClient.Namespace("test-namespace").List(context.Background(), metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Empty(t, list.Items)
}
