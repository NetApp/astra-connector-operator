package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/system"
	"github.com/jessevdk/go-flags"
	log "github.com/sirupsen/logrus"
	"golang.org/x/term"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

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

func getDockerPw() (string, error) {
	fmt.Print("Docker password: ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", nil
	}

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
		fmt.Println(scanner.Text())
		err := json.Unmarshal(scanner.Bytes(), &imageResponseBody)
		checkFatalErr(err)
		fmt.Printf("Debug marshall: %v\n", imageResponseBody)

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
		newTag := fmt.Sprintf("%s/%s", repoPrefix, image)

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

func pushImages(dockerClient *client.Client, opts *Options, images []string) error {
	if opts.ImageRepoUser == "" {
		var err error
		opts.ImageRepoUser, err = getDockerUsername()
		if err != nil {
			return err
		}
	}

	if opts.ImageRepoPw == "" {
		var err error
		opts.ImageRepoPw, err = getDockerPw()
		if err != nil {
			return err
		}
	}

	var authConfig = dockerTypes.AuthConfig{
		Username: opts.ImageRepoUser,
		Password: opts.ImageRepoPw,
	}
	authConfigBytes, err := json.Marshal(authConfig)
	if err != nil {
		return err
	}
	authConfigEncoded := base64.URLEncoding.EncodeToString(authConfigBytes)
	pushOpts := dockerTypes.ImagePushOptions{RegistryAuth: authConfigEncoded, All: true}

	for _, image := range images {
		log.Info("Pushing image")

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
		defer cancel()

		reader, err := dockerClient.ImagePush(ctx, image, pushOpts)
		defer reader.Close()
		if err != nil {
			return err
		}

		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			response := DockerPushResponse{}
			err := json.Unmarshal(scanner.Bytes(), &response)
			if err != nil {
				return err
			}

			if response.Error != "" {
				log.WithFields(log.Fields{
					"error":       response.Error,
					"errorDetail": *response.ErrorDetail,
				}).Error("Docker load error")
				return fmt.Errorf("error loading images")
			}

			log.WithFields(log.Fields{
				"status": response.Status,
				"id":     response.Id,
			}).Info("Pushing image...")
		}
	}
	return nil
}

type OperatorConfig struct {
	ApiVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`

	Metadata struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`

	Spec struct {
		NatssyncClient struct {
			Image string `yaml:"image"`
		} `yaml:"natssync-client,omitempty"`

		HttpProxyClient struct {
			Image string `yaml:"image"`
		} `yaml:"httpproxy-client,omitempty"`

		EchoClient struct {
			Image string `yaml:"image"`
		} `yaml:"echo-client,omitempty"`

		Nats struct {
			Image string `yaml:"image"`
		} `yaml:"nats,omitempty"`

		ImageRegistry struct {
			Name string `yaml:"name"`
		} `yaml:"imageRegistry,omitempty"`

		Astra struct {
			Token       string `yaml:"token"`
			ClusterName string `yaml:"clusterName"`
			AccountId   string `yaml:"accountId"`
			AcceptEula  string `yaml:"acceptEULA"`
		} `yaml:"astra"`
	} `yaml:"spec"`
}

func createNamespace(ns string) (string, error) {
	cmd := exec.Command("kubectl", "create", "ns", ns)
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
	ImageRepo         string `short:"r" long:"image-repo" required:"false" description:"Private Docker image repo URL" value-name:"URL"`
	ImageTar          string `short:"i" long:"image-tar" required:"false" description:"Path to image tar" value-name:"PATH"`
	ImageRepoUser     string `short:"u" long:"repo-user" required:"false" description:"Private Docker image repo URL" value-name:"USER"`
	ImageRepoPw       string `short:"p" long:"repo-pw" required:"false" description:"Private Docker image repo URL" value-name:"PASSWORD"`
	ClusterName       string `short:"c" long:"cluster-name" required:"true" description:"Private cluster name" value-name:"NAME"`
	RegisterToken     string `short:"t" long:"token" required:"true" description:"Astra API token" value-name:"TOKEN"`
	AcceptEula        bool   `long:"accept-eula" required:"true" description:"Accept the End User License Agreement"`
	AstraAccountId    string `short:"a" long:"account-id" required:"true" description:"Astra account ID" value-name:"ID"`
	Namespace         string `long:"namespace" required:"false" default:"astra-connector" description:"Astra Connector namespace" value-name:"NAMESPACE"`
	OperatorNamespace string `long:"operator-namespace" required:"false" default:"astra-connector-operator" description:"Astra Connector Operator namespace" value-name:"NAMESPACE"`
	AstraUrl          string `short:"x" long:"astra-url" required:"false" default:"https://eap.astra.netapp.io" description:"Url to Astra. E.g. 'https://integration.astra.netapp.io'" value-name:"URL"`
	SkipTlsValidation bool   `short:"z" long:"disable-tls" required:"false" description:"Disable TLS validation. TESTING ONLY."`
}

const (
	NatsImageName               = "nats"
	ConnectorDefaultsConfigPath = "./astraconnector_defaults.yaml"
	YamlOutputPath              = "./deployConfig.yaml"
	OperatorYamlPath            = "./astraconnector_operator.yaml"
)

func main() {
	var opts Options
	var opConfig OperatorConfig
	_, err := flags.Parse(&opts)
	checkFatalErr(err)

	absPath, err := filepath.Abs(ConnectorDefaultsConfigPath)
	checkFatalErr(err)
	yamlFile, err := ioutil.ReadFile(absPath)
	checkFatalErr(err)
	err = yaml.Unmarshal(yamlFile, &opConfig)
	checkFatalErr(err)
	test, _ := yaml.Marshal(opConfig)
	fmt.Printf("%s\n", test)

	if opts.ImageTar != "" && opts.ImageRepo != "" {
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
			output, err := pushCmd.CombinedOutput()
			if err != nil {
				msg := fmt.Sprintf(fmt.Sprint(err) + ": " + string(output))
				log.Fatal(msg)
			}
			log.Info(string(output))
		}

		opConfig.Spec.ImageRegistry.Name = opts.ImageRepo

		// Update nats image name. Other images use OperatorConfig.ImageRegistry.Name
		for _, image := range taggedImages {
			if strings.Contains(image, fmt.Sprintf("%s:", NatsImageName)) {
				opConfig.Spec.Nats.Image = image
			}
		}
	}

	// Update yaml data
	opConfig.Metadata.Namespace = opts.Namespace
	opConfig.Spec.Astra.AcceptEula = strconv.FormatBool(opts.AcceptEula)
	opConfig.Spec.Astra.AccountId = opts.AstraAccountId
	opConfig.Spec.Astra.ClusterName = opts.ClusterName
	opConfig.Spec.Astra.Token = opts.RegisterToken

	fmt.Printf("DEBUG: %v\n", opConfig)
	test2, _ := yaml.Marshal(opConfig)
	fmt.Printf("\n%s\n", test2)

	// Create namespaces
	log.Info("Creating Astra Controller namespace")
	output, err := createNamespace(opts.Namespace)
	checkFatalErr(err)
	log.Info(output)

	log.Info("Creating Astra Controller Operator namespace")
	output, err = createNamespace(opts.OperatorNamespace)
	checkFatalErr(err)
	log.Info(output)

	// Install Astra Controller Operator
	operatorYamlPath, err := filepath.Abs(OperatorYamlPath)
	applyCmd := exec.Command("kubectl", "apply", "-n", opts.OperatorNamespace, "-f", operatorYamlPath)
	output, err = runCmd(applyCmd)
	checkFatalErr(err)
	log.Info(output)

	// Write deployConfig.yaml file
	yamlData, err := yaml.Marshal(opConfig)
	yamlOutPath, err := filepath.Abs(YamlOutputPath)
	checkFatalErr(err)
	err = os.WriteFile(yamlOutPath, yamlData, 0644)
	checkFatalErr(err)

	// Install Astra Controller
	applyCmd = exec.Command("kubectl", "apply", "-f", yamlOutPath)
	output, err = runCmd(applyCmd)
	checkFatalErr(err)
	log.Info(output)
}
