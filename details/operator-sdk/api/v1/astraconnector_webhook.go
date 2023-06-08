// Copyright (c) 2023 NetApp, Inc. All Rights Reserved.

package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var astraConnectorLog = logf.Log.WithName("astra-connector-operator-resource")

func (ai *AstraConnector) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(ai).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-astra-netapp-io-v1-astraconnector,mutating=true,failurePolicy=fail,sideEffects=None,groups=astra.netapp.io,resources=astraconnectors,verbs=create;update,versions=v1,name=mastraconnector.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &AstraConnector{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (ai *AstraConnector) Default() {
	astraConnectorLog.Info("default", "name", ai.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-astra-netapp-io-v1-astraconnector,mutating=false,failurePolicy=fail,sideEffects=None,groups=astra.netapp.io,resources=astraconnectors,verbs=create;update,versions=v1,name=astraconnector.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &AstraConnector{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (ai *AstraConnector) ValidateCreate() error {
	astraConnectorLog.Info("validate create", "name", ai.Name)

	// TODO return errors from below
	_ = ai.ValidateCreateAstraConnector()
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (ai *AstraConnector) ValidateUpdate(old runtime.Object) error {
	astraConnectorLog.Info("validate update", "name", ai.Name)

	// TODO return errors from below
	_ = ai.ValidateUpdateAstraConnector()
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (ai *AstraConnector) ValidateDelete() error {
	return nil
}
