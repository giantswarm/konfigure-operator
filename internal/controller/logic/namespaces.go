package logic

import (
	"fmt"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"

	konfigurev1alpha1 "github.com/giantswarm/konfigure-operator/api/v1alpha1"
)

// maxQuotedValueLength caps how much of an invalid value ends up in an error message, so a huge
// value cannot push the status condition message past its length limit.
const maxQuotedValueLength = 100

// ParseList turns a comma-separated flag value into a slice, dropping empty entries and
// surrounding whitespace.
func ParseList(raw string) []string {
	var items []string

	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, item)
		}
	}

	return items
}

// ParsePrivilegedNamespaces turns the comma-separated --privileged-namespaces value into a slice.
func ParsePrivilegedNamespaces(raw string) []string {
	return ParseList(raw)
}

// NamespaceNotAllowedError is returned for a Konfiguration outside the privileged namespaces.
type NamespaceNotAllowedError struct {
	Namespace            string
	PrivilegedNamespaces []string
}

func (e *NamespaceNotAllowedError) Error() string {
	if len(e.PrivilegedNamespaces) == 0 {
		return fmt.Sprintf("Konfigurations are not reconciled in namespace %q: no privileged namespaces are configured", e.Namespace)
	}

	return fmt.Sprintf(
		"Konfigurations are only reconciled in the privileged namespaces %s, not in %q",
		strings.Join(e.PrivilegedNamespaces, ", "), e.Namespace,
	)
}

// InvalidReferenceError is returned for a Konfiguration whose references are not valid names.
type InvalidReferenceError struct {
	Violations []string
}

func (e *InvalidReferenceError) Error() string {
	return fmt.Sprintf("invalid references: %s", strings.Join(e.Violations, ", "))
}

// CheckReconcilable decides whether the operator may reconcile a Konfiguration at all. The
// operator renders with its own SOPS keys and writes ConfigMaps and Secrets in any namespace, so it
// only reconciles Konfigurations in the privileged namespaces, where everyone who can create one
// may also read what the operator can decrypt. Konfigurations there may reference any namespace.
//
// It returns a *NamespaceNotAllowedError or an *InvalidReferenceError, or nil.
func CheckReconcilable(cr *konfigurev1alpha1.Konfiguration, privilegedNamespaces []string) error {
	if !slices.Contains(privilegedNamespaces, cr.Namespace) {
		return &NamespaceNotAllowedError{Namespace: cr.Namespace, PrivilegedNamespaces: privilegedNamespaces}
	}

	// Names and namespaces end up in file system paths and API server URLs, so a value like
	// "../other-namespace/other-repository" must not get through. The CRD patterns may not be
	// installed yet or may not apply to objects created before them, so check here too.
	return validateReferenceNames(cr)
}

func validateReferenceNames(cr *konfigurev1alpha1.Konfiguration) error {
	references := []struct {
		field    string
		value    string
		validate func(string) []string
	}{
		{field: ".spec.sources.flux.gitRepository.name", value: cr.Spec.Sources.Flux.GitRepository.Name, validate: validation.IsDNS1123Subdomain},
		{field: ".spec.sources.flux.gitRepository.namespace", value: cr.Spec.Sources.Flux.GitRepository.Namespace, validate: validation.IsDNS1123Label},
		{field: ".spec.targets.schema.reference.name", value: cr.Spec.Targets.Schema.Reference.Name, validate: validation.IsDNS1123Subdomain},
		{field: ".spec.targets.schema.reference.namespace", value: cr.Spec.Targets.Schema.Reference.Namespace, validate: validation.IsDNS1123Label},
		{field: ".spec.destination.namespace", value: cr.Spec.Destination.Namespace, validate: validation.IsDNS1123Label},
	}

	var violations []string
	for _, ref := range references {
		if errs := ref.validate(ref.value); len(errs) > 0 {
			violations = append(violations, fmt.Sprintf("%s %q is not a valid name", ref.field, truncate(ref.value)))
		}
	}

	if len(violations) == 0 {
		return nil
	}

	return &InvalidReferenceError{Violations: violations}
}

func truncate(value string) string {
	if len(value) <= maxQuotedValueLength {
		return value
	}

	return value[:maxQuotedValueLength] + "..."
}

// ValidateNamespaceList checks that every entry of a namespace list from a flag is a valid
// namespace name.
func ValidateNamespaceList(namespaces []string) error {
	for _, namespace := range namespaces {
		if errs := validation.IsDNS1123Label(namespace); len(errs) > 0 {
			return fmt.Errorf("namespace %q is invalid: %s", namespace, strings.Join(errs, "; "))
		}
	}

	return nil
}
