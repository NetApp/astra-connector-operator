package util_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"

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

func TestIsValidDNS1123Label(t *testing.T) {
	tests := map[string]struct {
		input    string
		expected bool
	}{
		"input contains a number at end": {
			"valid-astra-cluster-input-0",
			true,
		},
		"input contains a number at beginning": {
			// "-" is not valid for end of Kubernetes names.
			"0-valid-astra-cluster-input",
			true,
		},
		"input is within the max character count": {
			rand.String(63),
			true,
		},
		"input is greater than the max character count": {
			rand.String(64),
			false,
		},
		"input is empty": {
			// "" is not valid for Kubernetes names.
			"",
			false,
		},
		"input contains illegal character at beginning": {
			// "-" is not valid for end of Kubernetes names.
			"-invalid-astra-cluster-input",
			false,
		},

		"input contains illegal character within": {
			// "_" is not valid for Kubernetes names.
			"invalid-astra-cluster_name",
			false,
		},
		"input contains illegal character at end": {
			// "-" is not valid for end of Kubernetes names.
			"invalid-astra-cluster-input-",
			false,
		},
		"input contains illegal uppercase character at beginning": {
			"Invalid-astra-cluster-input",
			false,
		},
		"input is a dns subdomain and not a input": {
			"example.com",
			false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, util.IsValidDNS1123Label(test.input))
		})
	}
}
