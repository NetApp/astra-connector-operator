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
)

func CheckFatalErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func getCreds(opts Options) (string, string, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter Username: ")
	username, err := reader.ReadString('\n')
	if err != nil {
		return "", "", err
	}

	fmt.Print("Enter Password: ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", "", err
	}

	password := string(bytePassword)
	return strings.TrimSpace(username), strings.TrimSpace(password), nil
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

	client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	CheckFatalErr(err)

	if opts.ImageTar != "" {
		// We use system.OpenSequential to use sequential file access on Windows, avoiding
		// depleting the standby list un-necessarily. On Linux, this equates to a regular os.Open.
		file, err := system.OpenSequential(opts.ImageTar)
		CheckFatalErr(err)
		defer file.Close()
		imageLoadResponse, err := client.ImageLoad(context.Background(), file, true)
		if err != nil {
			log.Fatal(err)
		}
		if imageLoadResponse.Body == nil {
			log.Fatal("Error: Docker load returned empty response")
		}
		if !imageLoadResponse.JSON {
			log.Fatal("Error: Docker response is in unknown format")
		}

		scanner := bufio.NewScanner(imageLoadResponse.Body)
		//var imageList []string
		type imageResponseBody struct {
			Stream      string `json:"stream"`
			Error       string `json:"error"`
			ErrorDetail struct {
				Message string `json:"message"`
			}
		}

		// Parse response, check for errors
		imageNames := map[string]string{}
		for scanner.Scan() {
			imageResponseBody := imageResponseBody{}
			fmt.Println(scanner.Text())
			err := json.Unmarshal([]byte(scanner.Text()), &imageResponseBody)
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
			imageName := strings.Split(strings.Trim(imageResponseBody.Stream, " \n"), ": ")
			// Map originalImageName:newImageName
			imageNames[imageName[1]] = fmt.Sprintf("%s/%s", opts.ImageRepo, imageName[1])
		}
		fmt.Printf("Image list: %v\n", imageNames)

		var authConfig = types.AuthConfig{
			Username: "",
			Password: "",
		}
		authConfigBytes, _ := json.Marshal(authConfig)
		authConfigEncoded := base64.URLEncoding.EncodeToString(authConfigBytes)
		opts := types.ImagePushOptions{RegistryAuth: authConfigEncoded}

		// Tag and push
		for srcImageName, newImageName := range imageNames {
			log.WithFields(log.Fields{
				"newTag":      newImageName,
				"sourceImage": srcImageName,
			}).Info("Re-tag image")
			err = client.ImageTag(context.Background(), srcImageName, newImageName)
			CheckFatalErr(err)
			log.Info("Pushing image")
			reader, err := client.ImagePush(context.Background(), newImageName, opts)
			var out []byte
			_, err = reader.Read(out)
			CheckFatalErr(err)
			fmt.Printf("out: %s\n", string(out))
			defer reader.Close()
			CheckFatalErr(err)
		}

		// Push images

		//edit yaml
	}

}
