package k8s_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/NetApp-Polaris/astra-connector-operator/details/k8s"
	"github.com/NetApp-Polaris/astra-connector-operator/mocks"
	testutil "github.com/NetApp-Polaris/astra-connector-operator/test/test-util"
)

func createHandler(t *testing.T) (k8s.ClusterTypeCheckerInterface, *mocks.K8sUtilInterface, kubernetes.Interface) {
	k8sUtil := new(mocks.K8sUtilInterface)
	mockInterface := fake.NewSimpleClientset()
	log := testutil.CreateLoggerForTesting(t)
	k8sUtil.On("K8sClientset").Return(mockInterface)
	clusterTypeChecker := k8s.NewClusterTypeChecker(k8sUtil, log)
	return clusterTypeChecker, k8sUtil, mockInterface
}

func TestDetermineClusterType(t *testing.T) {
	t.Run("AKS", func(t *testing.T) {
		clusterTypeChecker, k8sUtil, k8sInterface := createHandler(t)
		k8sUtil.On("RESTGet", mock.Anything).Return(nil, errors.New("testing"))
		k8sUtil.On("VersionGet").Return("1.1", nil)
		clusterRole := &v1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "aks-service",
			},
		}

		_, err := k8sInterface.RbacV1().ClusterRoles().Create(ctx, clusterRole, metav1.CreateOptions{})
		assert.NoError(t, err)

		assert.Equal(t, k8s.FlavorAKS, clusterTypeChecker.DetermineClusterType())
	})

	t.Run("Anthos", func(t *testing.T) {
		clusterTypeChecker, k8sUtil, _ := createHandler(t)
		k8sUtil.On("RESTGet", "apis/anthos.gke.io").Return(make([]byte, 0), nil)
		k8sUtil.On("VersionGet").Return("1.1", nil)

		assert.Equal(t, k8s.FlavorAnthos, clusterTypeChecker.DetermineClusterType())
	})

	t.Run("RKE", func(t *testing.T) {
		clusterTypeChecker, k8sUtil, _ := createHandler(t)
		k8sUtil.On("RESTGet", "apis/management.cattle.io").Return(make([]byte, 0), nil)
		k8sUtil.On("RESTGet", mock.Anything).Return(nil, errors.New("testing"))
		k8sUtil.On("VersionGet").Return("1.1", nil)

		assert.Equal(t, k8s.FlavorRKE, clusterTypeChecker.DetermineClusterType())
	})

	t.Run("RKE2", func(t *testing.T) {
		clusterTypeChecker, k8sUtil, _ := createHandler(t)
		k8sUtil.On("RESTGet", "apis/k3s.cattle.io").Return(make([]byte, 0), nil)
		k8sUtil.On("RESTGet", mock.Anything).Return(nil, errors.New("testing"))
		k8sUtil.On("VersionGet").Return("1.1", nil)

		assert.Equal(t, k8s.FlavorRKE2, clusterTypeChecker.DetermineClusterType())
	})

	t.Run("Tanzu", func(t *testing.T) {
		clusterTypeChecker, k8sUtil, _ := createHandler(t)
		k8sUtil.On("RESTGet", "apis/core.antrea.tanzu.vmware.com").Return(make([]byte, 0), nil)
		k8sUtil.On("RESTGet", mock.Anything).Return(nil, errors.New("testing"))
		k8sUtil.On("VersionGet").Return("1.1", nil)

		assert.Equal(t, k8s.FlavorTanzu, clusterTypeChecker.DetermineClusterType())
	})

	t.Run("GKE", func(t *testing.T) {
		clusterTypeChecker, k8sUtil, _ := createHandler(t)
		k8sUtil.On("RESTGet", mock.Anything).Return(nil, errors.New("testing"))
		k8sUtil.On("VersionGet").Return("1.1-gke", nil)

		assert.Equal(t, k8s.FlavorGKE, clusterTypeChecker.DetermineClusterType())
	})

	t.Run("openshift", func(t *testing.T) {
		clusterTypeChecker, k8sUtil, _ := createHandler(t)
		versionBytes := []byte(`{"Status": {"Versions": [{"Name": "openshift-apiserver", "Version": "1.0.0"}]}}`)

		k8sUtil.On("RESTGet", "apis/config.openshift.io/v1/clusteroperators/openshift-apiserver").Return(versionBytes, nil)
		k8sUtil.On("RESTGet", mock.Anything).Return(nil, errors.New("testing"))
		k8sUtil.On("VersionGet").Return("1.1", nil)

		assert.Equal(t, k8s.FlavorOpenShift, clusterTypeChecker.DetermineClusterType())
	})
}
