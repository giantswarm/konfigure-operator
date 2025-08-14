package logic

import (
	"errors"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	GeneratedByLabel      = KonfigureOperatorPrefix + "/generated-by"
	GeneratedByLabelValue = "konfigure-operator"

	OwnerApiGroupLabel   = KonfigureOperatorPrefix + "/ownerApiGroup"
	OwnerApiVersionLabel = KonfigureOperatorPrefix + "/ownerApiVersion"
	OwnerKindLabel       = KonfigureOperatorPrefix + "/ownerKind"
	OwnerNameLabel       = KonfigureOperatorPrefix + "/ownerName"
	OwnerNamespaceLabel  = KonfigureOperatorPrefix + "/ownerNamespace"
	RevisionLabel        = KonfigureOperatorPrefix + "/revision"
)

func GenerateOwnershipLabels(gvk schema.GroupVersionKind, meta v1.ObjectMeta, revision string) map[string]string {
	labels := map[string]string{}

	labels[GeneratedByLabel] = GeneratedByLabelValue

	labels[OwnerApiGroupLabel] = gvk.Group
	labels[OwnerApiVersionLabel] = gvk.Version
	labels[OwnerKindLabel] = gvk.Kind

	labels[OwnerNameLabel] = meta.Name
	labels[OwnerNamespaceLabel] = meta.Namespace

	// TODO This might require validation / sanitization if it is not guaranteed anymore to be a git commit hash
	labels[RevisionLabel] = revision

	return labels
}

// MatchOwnership Check all ownership labels except: api version (in case of CRD version bump)
// and revision of course.
func MatchOwnership(existing, desired v1.ObjectMeta) error {
	var labelMatchErrors []error

	for _, label := range []string{GeneratedByLabel, OwnerApiGroupLabel, OwnerKindLabel, OwnerNameLabel, OwnerNamespaceLabel} {
		if err := matchSingleLabel(label, existing, desired); err != nil {
			labelMatchErrors = append(labelMatchErrors, err)
		}
	}

	errorMessages := make([]string, len(labelMatchErrors))
	for _, err := range labelMatchErrors {
		errorMessages = append(errorMessages, err.Error())
	}

	if len(errorMessages) > 0 {
		return errors.New(strings.Join(errorMessages, "\n"))
	}

	return nil
}

func matchSingleLabel(key string, existing, desired v1.ObjectMeta) error {
	if existing.Labels[key] != desired.Labels[key] {
		return fmt.Errorf("label \"%s\" is set to \"%s\", expected to be: \"%s\"", key, existing.Labels[key], desired.Labels[key])
	}

	return nil
}
