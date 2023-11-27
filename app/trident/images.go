package trident

var imageMap = map[string]string{
	"acpImage":                             "netappdownloads.jfrog.io/oss-docker-trident-staging/astra/trident-acp:24.01.0-test.6506551416978f54458c89908e21826027b4570b",
	"tridentImage":                         "netappdownloads.jfrog.io/oss-docker-trident-staging/astra/trident:24.01.0-test.6506551416978f54458c89908e21826027b4570b",
	"tridentAutosupportImage":              "netapp/trident-autosupport:23.10.0",
	"tridentOperatorImage":                 "netappdownloads.jfrog.io/oss-docker-trident-staging/astra/trident-operator:24.01.0-test.6506551416978f54458c89908e21826027b4570b",
	"minSupportedBrownfieldTridentVersion": "23.01.0",
	"maxSupportedBrownfieldTridentVersion": "24.01.0-test.6506551416978f54458c89908e21826027b4570b",
	"gcpMinSupportKubernetesVersion":       "1.25.0-0",
	"gcpMaxSupportKubernetesVersion":       "1.28.99-0",
	"azureMinSupportKubernetesVersion":     "1.25.0-0",
	"azureMaxSupportKubernetesVersion":     "1.28.99-0",
}
