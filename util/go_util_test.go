package util_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
	"github.com/NetApp-Polaris/astra-connector-operator/util"
)

func createAstraConnector() *v1.AstraConnector {
	astraConnector := &v1.AstraConnector{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      "test-astra-connector",
			Namespace: "test-namespace",
		},
		Spec: v1.AstraConnectorSpec{
			Astra: v1.Astra{
				TokenRef:    "test-api-token",
				AccountId:   "test-account-id",
				ClusterName: "test-cluster-name",
			},
			AutoSupport: v1.AutoSupport{
				Enrolled: true,
				URL:      "https://my-asup",
			},
			NatsSyncClient: v1.NatsSyncClient{
				CloudBridgeURL: "test-url",
			},
		},
	}

	return astraConnector
}

func TestIsNil(t *testing.T) {
	t.Run("TestIsNil__NilInterfaceOrNilPointerReturnTrue", func(t *testing.T) {
		var i interface{}
		assert.Equal(t, true, util.IsNil(i))

		var p *int
		assert.Equal(t, true, util.IsNil(p))
	})

	t.Run("TestIsNil__NotNilInterfaceOrNotNilPointerReturnFalse", func(t *testing.T) {
		i := 42
		assert.Equal(t, false, util.IsNil(i))

		x := 10
		p := &x
		assert.Equal(t, false, util.IsNil(p))

		s := "Hello"
		assert.Equal(t, false, util.IsNil(s))
	})
}

func TestGetJSONFieldName(t *testing.T) {
	t.Run("TestGetJSONFieldName__WhenValidStructFieldReturnJSONTagForTheField", func(t *testing.T) {
		ac := createAstraConnector()

		jsonTag := util.GetJSONFieldName(&ac.Spec, &ac.Spec.Astra)
		assert.Equal(t, "astra", jsonTag)

		jsonTag = util.GetJSONFieldName(&ac.Status, &ac.Status.NatsSyncClient)
		assert.Equal(t, "natsSyncClient", jsonTag)
	})

	t.Run("TestGetJSONFieldName__WhenInvalidStructFieldReturnEmptyString", func(t *testing.T) {
		ac := createAstraConnector()

		type testData struct {
			field string
		}
		var a = testData{field: "value"}

		jsonTag := util.GetJSONFieldName(&ac.Spec, &a.field)
		assert.Equal(t, "", jsonTag)

		jsonTag = util.GetJSONFieldName(&ac.Status, &a.field)
		assert.Equal(t, "", jsonTag)
	})

	t.Run("TestGetJSONFieldName__WhenValidStructFieldIsNested", func(t *testing.T) {
		ac := createAstraConnector()

		jsonTag := util.GetJSONFieldName(&ac.Spec.Astra, &ac.Spec.Astra.ClusterName)
		assert.Equal(t, "clusterName", jsonTag)
	})
}

func TestIsValidKubernetesLabel(t *testing.T) {
	tests := map[string]struct {
		name     string
		expected bool
	}{
		"name contains a number at end": {
			"valid-astra-cluster-name-0",
			true,
		},
		"name contains a number at beginning": {
			// "-" is not valid for end of Kubernetes names.
			"0-valid-astra-cluster-name",
			true,
		},
		"name is greater than 64 characters": {
			fmt.Sprintf("resource-name%v", strings.Join(make([]string, 15), "-test")),
			false,
		},
		"name is empty": {
			// "" is not valid for Kubernetes names.
			"",
			false,
		},
		"name contains illegal character at beginning": {
			// "-" is not valid for end of Kubernetes names.
			"-invalid-astra-cluster-name",
			false,
		},

		"name contains illegal character within": {
			// "_" is not valid for Kubernetes names.
			"invalid-astra-cluster_name",
			false,
		},
		"name contains illegal character at end": {
			// "-" is not valid for end of Kubernetes names.
			"invalid-astra-cluster-name-",
			false,
		},
		"name contains uppercase illegal character at beginning": {
			// Uppercase letters are not valid for Kubernetes names.
			"Invalid-astra-cluster-name",
			false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			isValid := util.IsValidKubernetesLabel(test.name)
			assert.Equal(t, test.expected, isValid)
		})
	}
}
