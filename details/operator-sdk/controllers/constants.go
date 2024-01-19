package controllers

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// TODO: need to manually update this list if changes are made
var neptuneGVRs = []schema.GroupVersionResource{
	{
		Group:    "astra.netapp.io",
		Version:  "v1",
		Resource: "backupinplacerestores",
	},
	{
		Group:    "astra.netapp.io",
		Version:  "v1",
		Resource: "backuprestores",
	},
	{
		Group:    "astra.netapp.io",
		Version:  "v1",
		Resource: "appmirrorrelationships",
	},
	{
		Group:    "astra.netapp.io",
		Version:  "v1",
		Resource: "snapshotinplacerestores",
	},
	{
		Group:    "astra.netapp.io",
		Version:  "v1",
		Resource: "snapshotrestores",
	},
	{
		Group:    "astra.netapp.io",
		Version:  "v1",
		Resource: "backups",
	},
	{
		Group:    "astra.netapp.io",
		Version:  "v1",
		Resource: "snapshots",
	},
	{
		Group:    "astra.netapp.io",
		Version:  "v1",
		Resource: "applications",
	},
	{
		Group:    "astra.netapp.io",
		Version:  "v1",
		Resource: "appmirrorupdates",
	},
	{
		Group:    "astra.netapp.io",
		Version:  "v1",
		Resource: "autosupportbundles",
	},
	{
		Group:    "astra.netapp.io",
		Version:  "v1",
		Resource: "autosupportbundleschedules",
	},
	{
		Group:    "astra.netapp.io",
		Version:  "v1",
		Resource: "exechooks",
	},
	{
		Group:    "astra.netapp.io",
		Version:  "v1",
		Resource: "exechooksruns",
	},
	{
		Group:    "astra.netapp.io",
		Version:  "v1",
		Resource: "pvccopies",
	},
	{
		Group:    "astra.netapp.io",
		Version:  "v1",
		Resource: "pvcerases",
	},
	{
		Group:    "astra.netapp.io",
		Version:  "v1",
		Resource: "resourcedeletes",
	},
	{
		Group:    "astra.netapp.io",
		Version:  "v1",
		Resource: "resourcerestores",
	},
	{
		Group:    "astra.netapp.io",
		Version:  "v1",
		Resource: "resourcesummaryuploads",
	},
	{
		Group:    "astra.netapp.io",
		Version:  "v1",
		Resource: "resticvolumebackups",
	},
	{
		Group:    "astra.netapp.io",
		Version:  "v1",
		Resource: "resticvolumerestores",
	},
	{
		Group:    "astra.netapp.io",
		Version:  "v1",
		Resource: "schedules",
	},
	{
		Group:    "astra.netapp.io",
		Version:  "v1",
		Resource: "resourcebackups",
	},
	{
		Group:    "astra.netapp.io",
		Version:  "v1",
		Resource: "shutdownsnapshots",
	},
	{
		Group:    "astra.netapp.io",
		Version:  "v1",
		Resource: "appvaults",
	},
}
