package util_test

import (
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
				TokenRef:        "test-api-token",
				AccountId:       "test-account-id",
				ClusterName:     "test-cluster-name",
				AstraControlURL: "test-url",
			},
			AutoSupport: v1.AutoSupport{
				Enrolled: true,
				URL:      "https://my-asup",
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

		jsonTag = util.GetJSONFieldName(&ac.Status, &ac.Status)
		assert.Equal(t, "astraConnectorStatus", jsonTag)
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
}
