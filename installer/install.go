package main

import (
	"context"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/jessevdk/go-flags"
	"io/ioutil"
	"log"
)

func CheckFatalErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

type Options struct {
	//ClusterName       string `short:"c" long:"cluster-name" required:"true" description:"Private cluster name" value-name:"NAME"`
	//RegisterToken     string `short:"t" long:"token" required:"true" description:"Astra API token" value-name:"TOKEN"`
	//AstraAccountId    string `short:"a" long:"account-id" required:"true" description:"Astra account ID" value-name:"ID"`
	AstraUrl          string `short:"u" long:"astra-url" required:"false" default:"https://eap.astra.netapp.io" description:"Url to Astra. E.g. 'https://integration.astra.netapp.io'" value-name:"URL"`
	ImageRepo         string `short:"r" long:"image-repo" required:"false" description:"Private Docker image repo URL" value-name:"URL"`
	ImageTar          string `short:"it" long:"image-tar" required:"false" description:"Path to image tar" value-name:"PATH"`
	ImageRepoUser     string `short:"r" long:"repo-user" required:"false" description:"Private Docker image repo URL" value-name:"USER"`
	ImageRepoPw       string `short:"r" long:"repo-pw" required:"false" description:"Private Docker image repo URL" value-name:"PASSWORD"`
	SkipTlsValidation bool   `short:"z" long:"disable-tls" required:"false" description:"Disable TLS validation. TESTING ONLY."`
}

func main() {
	var opts Options
	_, err := flags.Parse(&opts)
	CheckFatalErr(err)

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	CheckFatalErr(err)

	if opts.ImageRepo != "" {
		// Get tar, load, etc

		//load images

		//push images

		//edit yaml
	}

	imageLoadResponse, err := cli.ImageLoad(context.Background(), input, true)
	if err != nil {
		log.Fatal(err)
	}
	body, err := ioutil.ReadAll(imageLoadResponse.Body)
	fmt.Println(string(body))

}
