// Copyright 2020 NetApp, Inc. All Rights Reserved.

package kubernetes

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"time"

	snapshot "github.com/kubernetes-csi/external-snapshotter/client/v6/clientset/versioned"
	torc "github.com/netapp/trident/operator/controllers/orchestrator/client/clientset/versioned"
	k8sversion "k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
)

type Clients struct {
	//KubeConfig *rest.Config
	KubeClient *kubernetes.Clientset
	K8SClient  Interface
	TorcClient *torc.Clientset
	SnapClient *snapshot.Clientset
	K8SVersion *k8sversion.Info
	Namespace  string
}

const k8sTimeout = 30 * time.Second

func CreateK8SClients(cfg *rest.Config, namespace string) (*Clients, error) {
	var clients *Clients
	var err error

	// Create the Kubernetes client
	clients.KubeClient, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	// Create the CRD client
	clients.TorcClient, err = torc.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("could not initialize Torc CRD client; %v", err)
	}

	// Create the Snapshot client
	clients.SnapClient, err = snapshot.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("could not initialize snapshot client; %v", err)
	}

	// Get the Kubernetes server version
	clients.K8SVersion, err = clients.KubeClient.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("could not get Kubernetes version: %v", err)
	}

	clients.Namespace = namespace
	log.WithFields(log.Fields{
		"namespace": clients.Namespace,
		"version":   clients.K8SVersion.String(),
	}).Info("Created Kubernetes clients.")

	return clients, nil
}

//
//	k8sPod := true
//	if _, err := os.Stat(config.NamespaceFile); os.IsNotExist(err) {
//		k8sPod = false
//	}
//
//	// Get the API config based on whether we are running in or out of cluster
//	if !k8sPod {
//		log.Debug("Creating ex-cluster Kubernetes clients.")
//		clients, err = createK8SClientsExCluster()
//	} else {
//		log.Debug("Creating in-cluster Kubernetes clients.")
//		clients, err = createK8SClientsInCluster()
//	}
//	if err != nil {
//		return nil, err
//	}
//
//	// Create the Kubernetes client
//	clients.KubeClient, err = kubernetes.NewForConfig(clients.KubeConfig)
//	if err != nil {
//		return nil, err
//	}
//
//	// Create the CRD client
//	clients.TorcClient, err = torc.NewForConfig(clients.KubeConfig)
//	if err != nil {
//		return nil, fmt.Errorf("could not initialize Torc CRD client; %v", err)
//	}
//
//	// Create the Snapshot client
//	clients.SnapClient, err = snapshot.NewForConfig(clients.KubeConfig)
//	if err != nil {
//		return nil, fmt.Errorf("could not initialize snapshot client; %v", err)
//	}
//
//	// Get the Kubernetes server version
//	clients.K8SVersion, err = clients.KubeClient.Discovery().ServerVersion()
//	if err != nil {
//		return nil, fmt.Errorf("could not get Kubernetes version: %v", err)
//	}
//
//	log.WithFields(log.Fields{
//		"namespace": clients.Namespace,
//		"version":   clients.K8SVersion.String(),
//	}).Info("Created Kubernetes clients.")
//
//	return clients, nil
//}
//
//func createK8SClientsExCluster() (*Clients, error) {
//
//	// Get K8S CLI
//
//	kubernetesCLI, err := discoverKubernetesCLI()
//	if err != nil {
//		return nil, err
//	}
//
//	// c.cli config view --raw
//	args := []string{"config", "view", "--raw"}
//
//	out, err := exec.Command(kubernetesCLI, args...).CombinedOutput()
//	if err != nil {
//		return nil, fmt.Errorf("%s; %v", string(out), err)
//	}
//
//	clientConfig, err := clientcmd.NewClientConfigFromBytes(out)
//	if err != nil {
//		return nil, err
//	}
//
//	restConfig, err := clientcmd.RESTConfigFromKubeConfig(out)
//	if err != nil {
//		return nil, err
//	}
//
//	namespace, _, err := clientConfig.Namespace()
//	if err != nil {
//		return nil, err
//	}
//
//	// Create the CLI-based Kubernetes client
//	k8sClient, err := NewKubeClient(restConfig, namespace, k8sTimeout)
//	if err != nil {
//		return nil, fmt.Errorf("could not initialize Kubernetes client; %v", err)
//	}
//
//	return &Clients{
//		KubeConfig: restConfig,
//		K8SClient:  k8sClient,
//		Namespace:  namespace,
//	}, nil
//}
//
//func createK8SClientsInCluster() (*Clients, error) {
//
//	kubeConfig, err := rest.InClusterConfig()
//	if err != nil {
//		return nil, err
//	}
//
//	// when running in a pod, we use the Trident pod's namespace
//	namespaceBytes, err := ioutil.ReadFile(config.NamespaceFile)
//	if err != nil {
//		return nil, fmt.Errorf("could not read namespace file %s; %v", config.NamespaceFile, err)
//	}
//	namespace := string(namespaceBytes)
//
//	// Create the Kubernetes client
//	k8sClient, err := NewKubeClient(kubeConfig, namespace, k8sTimeout)
//	if err != nil {
//		return nil, fmt.Errorf("could not initialize Kubernetes client; %v", err)
//	}
//
//	return &Clients{
//		KubeConfig: kubeConfig,
//		K8SClient:  k8sClient,
//		Namespace:  namespace,
//	}, nil
//}
//
//func CreateK8SClientsFromKubeConfig(kubeConfig []byte) (*Clients, error) {
//
//	clients := &Clients{}
//
//	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeConfig)
//	if err != nil {
//		return nil, err
//	}
//
//	clients.KubeConfig, err = clientcmd.RESTConfigFromKubeConfig(kubeConfig)
//	if err != nil {
//		return nil, err
//	}
//
//	clients.Namespace, _, err = clientConfig.Namespace()
//	if err != nil {
//		return nil, err
//	}
//
//	// Create the CLI-based Kubernetes client
//	clients.K8SClient, err = NewKubeClient(clients.KubeConfig, clients.Namespace, k8sTimeout)
//	if err != nil {
//		return nil, fmt.Errorf("could not initialize Kubernetes client; %v", err)
//	}
//
//	// Create the Kubernetes client
//	clients.KubeClient, err = kubernetes.NewForConfig(clients.KubeConfig)
//	if err != nil {
//		return nil, err
//	}
//
//	// Create the CRD client
//	clients.TorcClient, err = torc.NewForConfig(clients.KubeConfig)
//	if err != nil {
//		return nil, fmt.Errorf("could not initialize Torc CRD client; %v", err)
//	}
//
//	// Create the Snapshot client
//	clients.SnapClient, err = snapshot.NewForConfig(clients.KubeConfig)
//	if err != nil {
//		return nil, fmt.Errorf("could not initialize snapshot client; %v", err)
//	}
//
//	// Get the Kubernetes server version
//	clients.K8SVersion, err = clients.KubeClient.Discovery().ServerVersion()
//	if err != nil {
//		return nil, fmt.Errorf("could not get Kubernetes version: %v", err)
//	}
//
//	log.WithFields(log.Fields{
//		"namespace": clients.Namespace,
//		"version":   clients.K8SVersion.String(),
//	}).Info("Created Kubernetes clients.")
//
//	return clients, nil
//}
//
//func discoverKubernetesCLI() (string, error) {
//
//	// Try the OpenShift CLI first
//	_, err := exec.Command(CLIOpenShift, "version").Output()
//	if getExitCodeFromError(err) == ExitCodeSuccess {
//		return CLIOpenShift, nil
//	}
//
//	// Fall back to the K8S CLI
//	_, err = exec.Command(CLIKubernetes, "version").Output()
//	if getExitCodeFromError(err) == ExitCodeSuccess {
//		return CLIKubernetes, nil
//	}
//
//	if ee, ok := err.(*exec.ExitError); ok {
//		return "", fmt.Errorf("found the Kubernetes CLI, but it exited with error: %s",
//			strings.TrimRight(string(ee.Stderr), "\n"))
//	}
//
//	return "", fmt.Errorf("could not find the Kubernetes CLI: %v", err)
//}
//
//func getExitCodeFromError(err error) int {
//	if err == nil {
//		return ExitCodeSuccess
//	} else {
//
//		// Default to 1 in case we can't determine a process exit code
//		code := ExitCodeFailure
//
//		if exitError, ok := err.(*exec.ExitError); ok {
//			ws := exitError.Sys().(syscall.WaitStatus)
//			code = ws.ExitStatus()
//		}
//
//		return code
//	}
//}
