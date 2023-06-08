package controllers

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"github.com/NetApp-Polaris/astra-connector-operator/deployer/neptune"
	v1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

//go:embed yaml/neptune_crds.yaml
var f embed.FS

func (r *AstraConnectorController) deployNeptune(ctx context.Context,
	astraConnector *v1.AstraConnector, natssyncClientStatus *v1.NatssyncClientStatus) (ctrl.Result, error) {
	//// Install CRDs
	// Ran into cluster scope issue
	err := installCRDs(ctx, r)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Deploy Neptune
	neptuneDeployer := neptune.NewNeptuneClientDeployer()
	err = r.deployResources(ctx, neptuneDeployer, astraConnector, natssyncClientStatus)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func installCRDs(ctx context.Context, r *AstraConnectorController) error {
	log := ctrllog.FromContext(ctx)
	yamlFile, err := f.ReadFile("yaml/neptune_crds.yaml")
	if err != nil {
		return err
	}

	// Decode the YAML file into unstructured objects.
	dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	for _, rawObj := range bytes.Split(yamlFile, []byte("\n---\n")) {
		if len(bytes.TrimSpace(rawObj)) == 0 {
			continue
		}
		obj := &unstructured.Unstructured{}
		_, _, err = dec.Decode(rawObj, nil, obj)
		if err != nil {
			return err
		}

		key := client.ObjectKeyFromObject(obj)
		err = r.Client.Create(ctx, obj)
		if errors.IsAlreadyExists(err) {
			log.Info(fmt.Sprintf("CRD %s already exists\n", key.Name))
		} else if err != nil {
			return err
		}
	}
	return nil
}
