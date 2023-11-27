// Copyright 2020 NetApp, Inc. All Rights Reserved.

package model

type AstraLabel struct {
	Name  string `json:"name" bson:"name"`
	Value string `json:"value" bson:"value"`
}

// Metadata is just a simple convenience structure for our 4 "background" fields
type Metadata struct {
	CreationTimestamp     string       `json:"creationTimestamp" bson:"creationTimestamp"`
	ModificationTimestamp string       `json:"modificationTimestamp" bson:"modificationTimestamp"`
	DeletionTimestamp     string       `json:"deletionTimestamp" bson:"deletionTimestamp"`
	CreatedBy             string       `json:"createdBy" bson:"createdBy"`
	Labels                []AstraLabel `json:"labels,omitempty" bson:"labels,omitempty"`
}

func LabelsToMap(labels []AstraLabel) map[string]string {
	labelMap := make(map[string]string)

	for i := range labels {
		labelMap[labels[i].Name] = labels[i].Value
	}

	return labelMap
}
