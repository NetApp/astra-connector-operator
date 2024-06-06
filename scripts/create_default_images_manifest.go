/*
Used in GitHub actions to generate a manifest of default images deployed by connector-operator
*/
package main

import (
	"fmt"
	"os"

	"github.com/NetApp-Polaris/astra-connector-operator/common"
)

func main() {
	// Get cmd line args
	if len(os.Args) < 4 {
		fmt.Println("Usage: go run create_default_images_manifest.go <output_file_path> <version> <is-release>")
		os.Exit(1)
	}
	destinationFilePath := os.Args[1]
	connectorOperatorVersion := os.Args[2]
	isRelease := os.Args[3]

	fmt.Printf("Creating default-image manifest file %s\n", destinationFilePath)

	// Gather the full image name and versions
	defaultImageRegistry := ""
	if isRelease == "true" {
		defaultImageRegistry = common.AstraImageRegistry
	} else {
		defaultImageRegistry = common.DefaultImageRegistry
	}

	// Connector images
	images := []string{
		fmt.Sprintf("%s/astra-connector:%s", defaultImageRegistry, common.ConnectorImageTag),
		fmt.Sprintf("%s:%s", common.AstraConnectorOperatorRepository, connectorOperatorVersion),
		fmt.Sprintf("%s/trident-autosupport:%s", defaultImageRegistry, common.AsupImageTag),
		fmt.Sprintf("%s/%s", defaultImageRegistry, common.RbacProxyImage),
	}

	// Include Neptune related images
	for _, repository := range common.GetNeptuneRepositories() {
		images = append(images, fmt.Sprintf("%s/%s:%s", defaultImageRegistry, repository, common.NeptuneImageTag))
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
