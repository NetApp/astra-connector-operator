package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/system"
	"github.com/jessevdk/go-flags"
	log "github.com/sirupsen/logrus"
	"golang.org/x/term"
	"os"
	"strings"
	"syscall"
	"time"
)

func CheckFatalErr(err error) {
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
	CheckFatalErr(err)
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
		CheckFatalErr(err)
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

	var authConfig = types.AuthConfig{
		Username: opts.ImageRepoUser,
		Password: opts.ImageRepoPw,
	}
	fmt.Printf("debug auth: %v\n", authConfig)
	authConfigBytes, err := json.Marshal(authConfig)
	if err != nil {
		return err
	}
	authConfigEncoded := base64.URLEncoding.EncodeToString(authConfigBytes)
	pushOpts := types.ImagePushOptions{RegistryAuth: authConfigEncoded, All: true}

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
			}).Info("Pushing image...")
		}
	}
	return nil
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
	ImageRepo     string `short:"r" long:"image-repo" required:"false" description:"Private Docker image repo URL" value-name:"URL"`
	ImageTar      string `short:"i" long:"image-tar" required:"false" description:"Path to image tar" value-name:"PATH"`
	ImageRepoUser string `short:"u" long:"repo-user" required:"false" description:"Private Docker image repo URL" value-name:"USER"`
	ImageRepoPw   string `short:"p" long:"repo-pw" required:"false" description:"Private Docker image repo URL" value-name:"PASSWORD"`
	//ClusterName       string `short:"c" long:"cluster-name" required:"true" description:"Private cluster name" value-name:"NAME"`
	//RegisterToken     string `short:"t" long:"token" required:"true" description:"Astra API token" value-name:"TOKEN"`
	//AstraAccountId    string `short:"a" long:"account-id" required:"true" description:"Astra account ID" value-name:"ID"`
	AstraUrl          string `short:"x" long:"astra-url" required:"false" default:"https://eap.astra.netapp.io" description:"Url to Astra. E.g. 'https://integration.astra.netapp.io'" value-name:"URL"`
	SkipTlsValidation bool   `short:"z" long:"disable-tls" required:"false" description:"Disable TLS validation. TESTING ONLY."`
}

func main() {
	var opts Options
	_, err := flags.Parse(&opts)
	CheckFatalErr(err)

	if opts.ImageTar != "" && opts.ImageRepo != "" {
		dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		CheckFatalErr(err)

		log.Info("Loading image tar")
		loadedImages, err := loadImageTar(dockerClient, opts)
		CheckFatalErr(err)

		log.Info("Tagging images")
		taggedImages, err := tagImages(dockerClient, loadedImages, opts.ImageRepo)
		CheckFatalErr(err)

		log.Info("Pushing images")
		err = pushImages(dockerClient, &opts, taggedImages)
		CheckFatalErr(err)

	}

	//apply yaml

}
