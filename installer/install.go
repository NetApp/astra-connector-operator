package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/system"
	"github.com/jessevdk/go-flags"
	log "github.com/sirupsen/logrus"
	"golang.org/x/term"
	"gopkg.in/yaml.v2"
)

type ConnectorConfig struct {
	ApiVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`

	Metadata struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`

	Spec struct {
		NatssyncClient struct {
			Image             string `yaml:"image,omitempty"`
			CloudBridgeUrl    string `yaml:"cloud-bridge-url,omitempty"`
			HostAliasIP       string `yaml:"hostaliasIP,omitempty"`
			HostAlias         bool   `yaml:"hostalias,omitempty"`
			SkipTLSValidation bool   `yaml:"skipTLSValidation,omitempty"`
		} `yaml:"natssync-client"`

		HttpProxyClient struct {
			Image string `yaml:"image,omitempty"`
		} `yaml:"httpproxy-client"`

		EchoClient struct {
			Image string `yaml:"image,omitempty"`
		} `yaml:"echo-client"`

		Nats struct {
			Image string `yaml:"image,omitempty"`
		} `yaml:"nats"`

		ImageRegistry struct {
			Name string `yaml:"name,omitempty"`
		} `yaml:"imageRegistry"`

		Astra struct {
			Token       string `yaml:"token"`
			ClusterName string `yaml:"clusterName"`
			AccountId   string `yaml:"accountId"`
			AcceptEula  bool   `yaml:"acceptEULA"`
		} `yaml:"astra"`
	} `yaml:"spec"`
}

func checkFatalErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func getDockerUsername() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Docker username: ")
	username, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return username, nil
}

func getPw(prompt string) (string, error) {
	fmt.Print(prompt)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", nil
	}
	fmt.Println()
	return string(bytePassword), nil
}

func loadImageTar(dockerClient *client.Client, opts Options) ([]string, error) {
	// We use system.OpenSequential to use sequential file access on Windows, avoiding
	// depleting the standby list un-necessarily. On Linux, this equates to a regular os.Open.
	file, err := system.OpenSequential(opts.ImageTar)
	checkFatalErr(err)
	defer file.Close()

	imageLoadResponse, err := dockerClient.ImageLoad(context.Background(), file, true)
	if err != nil {
		log.Fatal(err)
	}
	if imageLoadResponse.Body == nil {
		return nil, fmt.Errorf("docker load returned empty response")
	}
	if !imageLoadResponse.JSON {
		return nil, fmt.Errorf("docker response is in unknown format")
	}

	var imageList []string
	// Parse response, check for errors
	scanner := bufio.NewScanner(imageLoadResponse.Body)
	for scanner.Scan() {
		imageResponseBody := DockerLoadResponse{}
		log.Info(scanner.Text())
		err := json.Unmarshal(scanner.Bytes(), &imageResponseBody)
		checkFatalErr(err)

		// Check response error
		if imageResponseBody.Error != "" {
			log.WithFields(log.Fields{
				"error":       imageResponseBody.Error,
				"errorDetail": imageResponseBody.ErrorDetail,
			}).Fatal("Docker load error")
		}

		// Get loaded image names
		imageName := strings.Split(strings.Trim(imageResponseBody.Stream, " \n"), ": ")[1]
		imageList = append(imageList, imageName)
		log.WithFields(log.Fields{
			"image": imageName,
		}).Info("Loaded image")
	}
	return imageList, nil
}

func tagImages(dockerClient *client.Client, images []string, repoPrefix string) ([]string, error) {
	var imagesWithRepo []string
	for _, image := range images {
		//todo: update operator to leave the repo in place)E.g. 'myPrivateRepo/asdf.com/theotw/echo-proxylet:1.2.3'
		newTag := fmt.Sprintf("%s/%s", repoPrefix, strings.TrimPrefix(image, "theotw/"))

		log.WithFields(log.Fields{
			"newTag":      newTag,
			"sourceImage": image,
		}).Info("Re-tag image")

		err := dockerClient.ImageTag(context.Background(), image, newTag)
		if err != nil {
			return nil, err
		}
		imagesWithRepo = append(imagesWithRepo, newTag)
	}
	return imagesWithRepo, nil
}

////todo: fix auth issue here so we can stop pushing via system call
//func pushImages(dockerClient *client.Client, opts *Options, images []string) error {
//	if opts.ImageRepoUser == "" {
//		var err error
//		opts.ImageRepoUser, err = getDockerUsername()
//		if err != nil {
//			return err
//		}
//	}
//
//	if opts.ImageRepoPw == "" {
//		var err error
//		opts.ImageRepoPw, err = getDockerPw()
//		if err != nil {
//			return err
//		}
//	}
//
//	var authConfig = dockerTypes.AuthConfig{
//		Username: opts.ImageRepoUser,
//		Password: opts.ImageRepoPw,
//	}
//	authConfigBytes, err := json.Marshal(authConfig)
//	if err != nil {
//		return err
//	}
//	authConfigEncoded := base64.URLEncoding.EncodeToString(authConfigBytes)
//	pushOpts := dockerTypes.ImagePushOptions{RegistryAuth: authConfigEncoded, All: true}
//
//	for _, image := range images {
//		log.Info("Pushing image")
//
//		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
//		defer cancel()
//
//		reader, err := dockerClient.ImagePush(ctx, image, pushOpts)
//		defer reader.Close()
//		if err != nil {
//			return err
//		}
//
//		scanner := bufio.NewScanner(reader)
//		for scanner.Scan() {
//			response := DockerPushResponse{}
//			err := json.Unmarshal(scanner.Bytes(), &response)
//			if err != nil {
//				return err
//			}
//
//			if response.Error != "" {
//				log.WithFields(log.Fields{
//					"error":       response.Error,
//					"errorDetail": *response.ErrorDetail,
//				}).Error("Docker load error")
//				return fmt.Errorf("error loading images")
//			}
//
//			log.WithFields(log.Fields{
//				"status": response.Status,
//				"id":     response.Id,
//			}).Info("Pushing image...")
//		}
//	}
//	return nil
//}

func createNamespace(ns string) (string, error) {
	cmd := exec.Command("kubectl", "create", "ns", ns)
	cmd.Env = os.Environ()
	return runCmd(cmd)
}

func runCmd(cmd *exec.Cmd) (string, error) {
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%s: %s", err, output)
	}
	return string(output), nil
}

type ErrorDetail struct {
	Message string `json:"message"`
}

type DockerPushResponse struct {
	Status      string       `json:"status"`
	Id          string       `json:"id"`
	Error       string       `json:"error"`
	ErrorDetail *ErrorDetail `json:"errorDetail"`
}

type DockerLoadResponse struct {
	Stream      string       `json:"stream"`
	Error       string       `json:"error"`
	ErrorDetail *ErrorDetail `json:"errorDetail"`
}

type Options struct {
	ImageRepo string `short:"r" long:"image-repo" required:"false" description:"Private Docker image repo URL" value-name:"URL"`
	ImageTar  string `short:"p" long:"image-tar" required:"false" description:"Path to image tar" value-name:"PATH"`
	//ImageRepoUser     string `long:"repo-user" required:"false" description:"Private Docker image repo URL" value-name:"USER"`
	//ImageRepoPw       string `long:"repo-pw" required:"false" description:"Private Docker image repo URL" value-name:"PASSWORD"`
	ClusterName       string `short:"c" long:"cluster-name" required:"false" description:"Private cluster name" value-name:"NAME"`
	Token             string `short:"t" long:"token" required:"true" description:"Astra API token" value-name:"TOKEN"`
	AcceptEula        bool   `long:"accept-eula" required:"true" description:"(flag) Accept End User License Agreement"`
	AstraAccountId    string `short:"a" long:"account-id" required:"true" description:"Astra account ID" value-name:"ID"`
	AstraUrl          string `short:"u" long:"astra-url" required:"false" default:"https://eap.astra.netapp.io" description:"Url to Astra. E.g. 'https://integration.astra.netapp.io'" value-name:"URL"`
	SkipTlsValidation bool   `short:"z" long:"disable-tls" required:"false" description:"(flag) Disable TLS validation. TESTING ONLY."`
	HostAliasIP       string `long:"hostAliasIP" required:"false" description:"The IP of the Astra host. TESTING ONLY." value-name:"IP"`
	HostAlias         bool   `long:"hostAlias" required:"false" description:"(flag) Set to enable HostAliasIP. TESTING ONLY"`
}

const (
	NatsImageName               = "nats"
	ConnectorDefaultsConfigPath = "./controllerconfig.yaml"
	YamlOutputPath              = "./deployConfig.yaml"
	OperatorYamlPath            = "./astraconnector_operator.yaml"
	ConnectorNamespace          = "astra-connector"
	ConnectorOperatorNamespace  = "astra-connector-operator"
)

func main() {
	formatter := &log.TextFormatter{
		FullTimestamp: true,
	}
	log.SetFormatter(formatter)

	var opts Options
	var connectorConfig ConnectorConfig
	_, err := flags.Parse(&opts)
	checkFatalErr(err)

	log.Info("Installing Astra Connector")

	absPath, err := filepath.Abs(ConnectorDefaultsConfigPath)
	checkFatalErr(err)
	yamlFile, err := ioutil.ReadFile(absPath)
	checkFatalErr(err)
	err = yaml.Unmarshal(yamlFile, &connectorConfig)
	checkFatalErr(err)

	// input check
	if opts.Token == "" {
		opts.Token, err = getPw("Enter Astra API token:")
		checkFatalErr(err)
	}

	// input check
	if opts.ImageTar != "" || opts.ImageRepo != "" {
		if !(opts.ImageTar != "" && opts.ImageRepo != "") {
			log.Fatal("error: 'image-tar' and 'image-repo' options must be used together")
		}
	}

	// input check
	if opts.HostAlias {
		if opts.HostAliasIP == "" {
			log.Fatal("error: 'hostAliasIP' must be defined if the 'hostAlias' flag is set ")
		}
	}

	if opts.ImageTar != "" {
		dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		checkFatalErr(err)

		log.Info("Loading image tar")
		loadedImages, err := loadImageTar(dockerClient, opts)
		checkFatalErr(err)

		log.Info("Tagging images")
		taggedImages, err := tagImages(dockerClient, loadedImages, opts.ImageRepo)
		checkFatalErr(err)

		//todo: Fix the auth issue preventing this from succeeding
		//log.Info("Pushing images")
		//err = pushImages(dockerClient, &opts, taggedImages)
		//checkFatalErr(err)

		// This is a workaround until we can push using the docker package
		for _, image := range taggedImages {
			pushCmd := exec.Command("docker", "push", image)
			pushCmd.Env = os.Environ()
			output, err := pushCmd.CombinedOutput()
			if err != nil {
				msg := fmt.Sprintf(fmt.Sprint(err) + ": " + string(output))
				log.Fatal(msg)
			}
			log.Info(string(output))
		}

		connectorConfig.Spec.ImageRegistry.Name = opts.ImageRepo
	}

	// Update yaml data
	connectorConfig.Metadata.Namespace = ConnectorNamespace
	connectorConfig.Spec.Astra.AcceptEula = opts.AcceptEula
	connectorConfig.Spec.Astra.AccountId = opts.AstraAccountId
	connectorConfig.Spec.Astra.ClusterName = opts.ClusterName
	connectorConfig.Spec.Astra.Token = opts.Token
	connectorConfig.Spec.NatssyncClient.SkipTLSValidation = opts.SkipTlsValidation
	connectorConfig.Spec.NatssyncClient.CloudBridgeUrl = opts.AstraUrl
	connectorConfig.Spec.NatssyncClient.HostAliasIP = opts.HostAliasIP
	connectorConfig.Spec.NatssyncClient.HostAlias = opts.HostAlias

	// Create namespaces
	log.Info("Creating Astra Connector namespace")
	output, err := createNamespace(ConnectorNamespace)
	checkFatalErr(err)
	log.Info(output)

	log.Info("Creating Astra Connector Operator namespace")
	output, err = createNamespace(ConnectorOperatorNamespace)
	checkFatalErr(err)
	log.Info(output)

	// Install Astra Connector Operator
	operatorYamlPath, err := filepath.Abs(OperatorYamlPath)
	applyCmd := exec.Command("kubectl", "apply", "-n", ConnectorOperatorNamespace, "-f", operatorYamlPath)
	applyCmd.Env = os.Environ()
	output, err = runCmd(applyCmd)
	checkFatalErr(err)
	log.Info(output)

	// Write deployConfig.yaml file
	yamlData, err := yaml.Marshal(connectorConfig)
	yamlOutPath, err := filepath.Abs(YamlOutputPath)
	checkFatalErr(err)
	err = os.WriteFile(yamlOutPath, yamlData, 0644)
	checkFatalErr(err)

	// Log yaml
	configBytes, err := yaml.Marshal(connectorConfig)
	checkFatalErr(err)
	configStr := string(configBytes)
	regEx := regexp.MustCompile(`( +token:)(.*)`)
	configStr = regEx.ReplaceAllString(configStr, "$1 *******") // don't show token
	log.Info("Applying AstraConnector yaml")
	log.Info(configStr)

	// Install Astra Connector
	applyCmd = exec.Command("kubectl", "apply", "-f", yamlOutPath)
	applyCmd.Env = os.Environ()
	output, err = runCmd(applyCmd)
	checkFatalErr(err)
	log.Info(output)

	log.Info("Astra Connector and Astra Connector Operator have been successfully applied")
	log.Info("Installation finishing in background")
}
