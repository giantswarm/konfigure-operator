package logic

import (
	"github.com/giantswarm/konfigure-operator/api/v1alpha1"
	"strings"
)

const (
	GeneratedByLabel     = "configuration.giantswarm.io/generated-by"
	OwnerApiGroupLabel   = "configuration.giantswarm.io/ownerApiGroup"
	OwnerApiVersionLabel = "configuration.giantswarm.io/ownerApiVersion"
	OwnerKindLabel       = "configuration.giantswarm.io/ownerKind"
	OwnerNameLabel       = "configuration.giantswarm.io/ownerName"
	OwnerNamespaceLabel  = "configuration.giantswarm.io/ownerNamespace"
	RevisionLabel        = "configuration.giantswarm.io/revision"
)

func GenerateOwnershipLabels(cr *v1alpha1.ManagementClusterConfiguration, revision string) map[string]string {
	labels := map[string]string{}

	// Label values cannot contain slashes
	var group, version string
	splitApiVersion := strings.Split(cr.APIVersion, "/")
	if len(splitApiVersion) == 1 {
		group = ""
		version = splitApiVersion[0]
	} else if len(splitApiVersion) == 2 {
		group = splitApiVersion[0]
		version = splitApiVersion[1]
	} else {
		group = "unknown"
		version = "unknown"
	}

	labels[GeneratedByLabel] = "konfigure-operator"

	labels[OwnerApiGroupLabel] = group
	labels[OwnerApiVersionLabel] = version
	labels[OwnerKindLabel] = cr.Kind

	labels[OwnerNameLabel] = cr.Name
	labels[OwnerNamespaceLabel] = cr.Namespace

	// TODO This might require validation / sanitization if it is not guaranteed anymore to be a git commit hash
	labels[RevisionLabel] = revision

	return labels
}
