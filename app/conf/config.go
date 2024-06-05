// Copyright (c) 2023 NetApp, Inc. All Rights Reserved.

package conf

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

var Config *ImmutableConfiguration = nil
var runAsUser = pointer.Int64(10001)
var runAsGroup = pointer.Int64(20000)

/*
ImmutableConfiguration

If you add or change a Config option, you must set it in 4 places (which is annoying):
 1. The ImmutableObject Config
    i. Struct field
    ii. Getter functions. The private MutableConfiguration struct as a Public field
 3. The DefaultConfiguration() method
 4. The toImmutableConfig() conversion constructor.
*/
type ImmutableConfiguration struct {
	host                    string
	appRoot                 string
	port                    int
	metricsPort             int
	healthProbePort         int
	waitDurationForResource time.Duration
	errorTimeout            time.Duration
	featureFlags            ImmutableFeatureFlags

	// This is only stored to be able to log it at app start-up: Do not use this field it is not immutable
	config *MutableConfiguration
}

type MutableConfiguration struct {
	// Set HOST = localhost in your development environment, this allows the server to
	// start on the localhost instead of trying to open an external port, avoiding the annoying pop up
	// on Mac development environment when the controller app is started.
	Host    string
	AppRoot string
	// Changing the ports should only be used for local development because actually changing the defaults on the deployed
	// operator requires additional yaml changes. But we need a local development override because
	// mcafee takes the default HealthPortProbe on Mac.
	Port                    int
	MetricsPort             int
	HealthProbePort         int
	WaitDurationForResource time.Duration
	ErrorTimeout            time.Duration
	FeatureFlags            featureFlags
}

// DefaultConfiguration Returns a MutableConfiguration that holds all the default values to be used,
// if they are not overridden by env or .config.yaml.
func DefaultConfiguration() *MutableConfiguration {
	applicationRoot := appRoot()

	return &MutableConfiguration{
		Host:                    "", // operator-sdk uses empty string, so defaulting to empty string
		AppRoot:                 applicationRoot,
		Port:                    9443,
		MetricsPort:             8080,
		HealthProbePort:         8081,
		WaitDurationForResource: 5 * time.Minute,
		ErrorTimeout:            5,
		FeatureFlags: featureFlags{
			DeployConnector: true,
			DeployNeptune:   true,
		},
	}
}

// toImmutableConfig Creates an ImmutableConfiguration from a MutableConfiguration object.
func toImmutableConfig(config *MutableConfiguration) *ImmutableConfiguration {
	immutableConfig := &ImmutableConfiguration{
		host:                    config.Host,
		appRoot:                 config.AppRoot,
		port:                    config.Port,
		metricsPort:             config.MetricsPort,
		healthProbePort:         config.HealthProbePort,
		waitDurationForResource: config.WaitDurationForResource,
		errorTimeout:            config.ErrorTimeout,
		featureFlags: ImmutableFeatureFlags{
			deployConnector: config.FeatureFlags.DeployConnector,
			deployNeptune:   config.FeatureFlags.DeployNeptune,
		},
		config: config,
	}

	return immutableConfig
}

// LogCurrentConfig This is used to log the ImmutableConfiguration object after it has been loaded without having to expose any internal mutable fields.
func (i ImmutableConfiguration) LogCurrentConfig(log logr.Logger, msg string) {
	log.Info(msg, "config", i.config)
}

func (i ImmutableConfiguration) Host() string {
	return i.host
}

func (i ImmutableConfiguration) AppRoot() string {
	return i.appRoot
}

func (i ImmutableConfiguration) Port() int {
	return i.port
}

func (i ImmutableConfiguration) MetricsPort() int {
	return i.metricsPort
}

func (i ImmutableConfiguration) HealthProbePort() int {
	return i.healthProbePort
}

func (i ImmutableConfiguration) WaitDurationForResource() time.Duration {
	return i.waitDurationForResource
}

func (i ImmutableConfiguration) ErrorTimeout() time.Duration {
	return i.errorTimeout
}

func (i ImmutableConfiguration) FeatureFlags() ImmutableFeatureFlags {
	return i.featureFlags
}

type ImmutableFeatureFlags struct {
	deployConnector bool
	deployNeptune   bool
}

type featureFlags struct {
	DeployConnector bool
	DeployNeptune   bool
}

func (f ImmutableFeatureFlags) DeployConnector() bool {
	return f.deployConnector
}

func (f ImmutableFeatureFlags) DeployNeptune() bool {
	return f.deployNeptune
}

// Viper configuration
func init() {
	Config = toImmutableConfig(load())
}

// load loads the viper config with the provision of values being overridden by either a .config.yaml file or an ENV var.
func load() *MutableConfiguration {
	v := viper.New()

	// Viper needs to know if a key exists in order to override it, so ~ in the DefaultConfiguration. This does two things:
	// 1) Makes Viper aware of all keys
	// 2) Provides the default values.
	dcYamlBytes, err := yaml.Marshal(DefaultConfiguration()) // Marshall the DefaultConfiguration to yaml
	if err != nil {
		panic(fmt.Errorf("Fatal error unable to marshall app default config: %w \n", err))
	}

	defaultConfig := bytes.NewReader(dcYamlBytes)
	v.SetConfigType("yaml")
	if err := v.MergeConfig(defaultConfig); err != nil { // Read the DefaultConfiguration yaml into a Viper config.
		panic(fmt.Errorf("Fatal error unable to merge app default config: %w \n", err))
	}

	// Override default values with values from .config.yaml if they exist
	v.SetConfigFile(appRoot() + "/.config.yaml")
	if err := v.MergeInConfig(); err != nil {
		// Providing a bad .config.yaml file will cause a panic, so you can find out what's wrong with it, but since
		// the .config.yaml file is optional, a missing file will not panic.
		if errors.As(err, &viper.ConfigParseError{}) {
			panic(fmt.Errorf("Fatal error unable to merge app .config.yaml: %w \n", err))
		}
	}

	// Override default values or .config.yaml values with ENV vars if they exist.
	// All ENV variables should be prefixed with ACOP (=> Astra Connector Operator)
	// Nested Config options should use an _ between object keys: e.g. ACOP_FEATUREFLAGS_DEPLOYNEPTUNE
	v.AutomaticEnv()
	v.SetEnvPrefix("ACOP")

	// This allows nested ENV keys to use _
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Create and return a new Configuration pointer with all the merged values
	config := &MutableConfiguration{}
	err = v.Unmarshal(config)
	if err != nil {
		panic(fmt.Errorf("Fatal error unable to unmarshall app config: %w \n", err))
	}

	return config
}

// appRoot Returns the root directory of the application
func appRoot() string {
	// path to 'this' config.go file which we know is in the app/conf dir
	_, configGoFile, _, _ := runtime.Caller(0)

	cwd, _ := os.Getwd()
	appDir := filepath.Join(filepath.Dir(configGoFile), "..") // e.g. /astra-connector-operator/app
	appParentDir := filepath.Join(appDir, "..")               // e.g. /astra-connector-operator

	if strings.Contains(appParentDir, "astra-connector-operator") { // TODO - this needs to be updated when the names of the operator is changed to astra-connector or some 'X'
		return appParentDir
	} else {
		return cwd
	}
}

func GetRunAsUser() *int64 {
	return runAsUser
}

func GetRunAsGroup() *int64 {
	return runAsGroup
}

var sc = corev1.SecurityContext{
	ReadOnlyRootFilesystem: pointer.Bool(true),
	RunAsNonRoot:           pointer.Bool(true),
	RunAsUser:              GetRunAsUser(),
	RunAsGroup:             GetRunAsGroup(),
}

func GetSecurityContext() *corev1.SecurityContext {
	return &sc
}

var pc = corev1.PodSecurityContext{
	RunAsNonRoot: pointer.Bool(true),
}

func GetPodSecurityContext() *corev1.PodSecurityContext {
	return &pc
}
