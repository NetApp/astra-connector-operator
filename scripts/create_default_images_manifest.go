/*
Used in GitHub actions to generate a manifest of default images deployed by connector-operator
*/
package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/NetApp-Polaris/astra-connector-operator/common"
)

func main() {
	// Get cmd line args
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run create_default_images_manifest.go <output_file_path> <version>")
		os.Exit(1)
	}
	destinationFilePath := os.Args[1]
	connectorOperatorVersion := os.Args[2]

	fmt.Printf("Creating default-image manifest file %s\n", destinationFilePath)

	// Gather the full image name and versions
	defaultImageRegistry := common.DefaultImageRegistry
	neptuneTag := getNeptuneTag()

	// Connector images
	images := []string{
		fmt.Sprintf("%s/%s", defaultImageRegistry, common.AstraConnectDefaultImage),
		fmt.Sprintf("%s/%s", defaultImageRegistry, common.NatsSyncClientDefaultImage),
		fmt.Sprintf("%s/%s", defaultImageRegistry, common.NatsDefaultImage),
		fmt.Sprintf("%s:%s", common.AstraConnectorOperatorRepository, connectorOperatorVersion),
	}

	// Include Neptune related images
	for _, repository := range common.GetNeptuneRepositories() {
		images = append(images, fmt.Sprintf("%s/%s:%s", defaultImageRegistry, repository, neptuneTag))
	}

	// Open the manifest file
	file, err := os.OpenFile(destinationFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Printf("error opening output file for writing: %s\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// Write the file
	for _, image := range images {
		// Write the string content to the file
		_, err = file.WriteString(fmt.Sprintf("%s\n", image))
		if err != nil {
			fmt.Printf("error writing to file: %s\n", err)
			os.Exit(1)
		}
	}
}

func getNeptuneTag() string {
	neptuneTag := common.NeptuneDefaultTag
	file, err := os.Open(common.NeptuneTagFile)
	if err != nil {
		fmt.Printf("error opening file to get neptune tag")
		os.Exit(2)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// scan the contents of the file and return first line
	// neptune manager tag is present in the first line
	for scanner.Scan() {
		neptuneTag = scanner.Text()
		fmt.Println("neptune manager tag : " + neptuneTag)
		return neptuneTag
	}

	fmt.Println("Using default neptune tag, error reading from file")
	return neptuneTag
}
