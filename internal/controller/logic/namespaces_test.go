package logic

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	konfigurev1alpha1 "github.com/giantswarm/konfigure-operator/api/v1alpha1"
)

func konfiguration(namespace, gitRepositoryNamespace, schemaNamespace, destinationNamespace string) *konfigurev1alpha1.Konfiguration {
	return &konfigurev1alpha1.Konfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: namespace},
		Spec: konfigurev1alpha1.KonfigurationSpec{
			Sources: konfigurev1alpha1.Sources{
				Flux: konfigurev1alpha1.FluxSource{
					GitRepository: konfigurev1alpha1.FluxSourceGitRepository{Name: "repo", Namespace: gitRepositoryNamespace},
				},
			},
			Targets: konfigurev1alpha1.Targets{
				Schema: konfigurev1alpha1.Schema{
					Reference: konfigurev1alpha1.SchemaReference{Name: "schema", Namespace: schemaNamespace},
				},
			},
			Destination: konfigurev1alpha1.Destination{Namespace: destinationNamespace},
		},
	}
}

func withGitRepositoryName(cr *konfigurev1alpha1.Konfiguration, name string) *konfigurev1alpha1.Konfiguration {
	cr.Spec.Sources.Flux.GitRepository.Name = name
	return cr
}

func withSchemaName(cr *konfigurev1alpha1.Konfiguration, name string) *konfigurev1alpha1.Konfiguration {
	cr.Spec.Targets.Schema.Reference.Name = name
	return cr
}

func TestCheckReconcilable(t *testing.T) {
	privileged := []string{"giantswarm", "flux-giantswarm"}

	testCases := []struct {
		name       string
		cr         *konfigurev1alpha1.Konfiguration
		privileged []string
		// wantNamespaceErr expects a *NamespaceNotAllowedError.
		wantNamespaceErr bool
		// wantFields expects an *InvalidReferenceError naming these fields.
		wantFields []string
	}{
		{
			name:       "privileged namespace may reference other namespaces",
			cr:         konfiguration("flux-giantswarm", "flux-giantswarm", "giantswarm", "kube-system"),
			privileged: privileged,
		},
		{
			name:       "privileged namespace referencing itself",
			cr:         konfiguration("giantswarm", "giantswarm", "giantswarm", "giantswarm"),
			privileged: privileged,
		},
		{
			name:             "unprivileged namespace is refused even with same-namespace references",
			cr:               konfiguration("org-acme", "org-acme", "org-acme", "org-acme"),
			privileged:       privileged,
			wantNamespaceErr: true,
		},
		{
			name:             "unprivileged namespace referencing a privileged one is refused",
			cr:               konfiguration("org-acme", "flux-giantswarm", "giantswarm", "org-acme"),
			privileged:       privileged,
			wantNamespaceErr: true,
		},
		{
			name:             "no privileged namespaces refuses everything",
			cr:               konfiguration("giantswarm", "flux-giantswarm", "giantswarm", "giantswarm"),
			privileged:       nil,
			wantNamespaceErr: true,
		},
		{
			name:             "namespace is checked before names",
			cr:               withGitRepositoryName(konfiguration("org-acme", "org-acme", "org-acme", "org-acme"), "../x"),
			privileged:       privileged,
			wantNamespaceErr: true,
		},
		{
			name:       "path traversal in the git repository name is rejected",
			cr:         withGitRepositoryName(konfiguration("giantswarm", "flux-giantswarm", "giantswarm", "giantswarm"), "../flux-giantswarm/giantswarm-config"),
			privileged: privileged,
			wantFields: []string{".spec.sources.flux.gitRepository.name"},
		},
		{
			name:       "invalid schema name is rejected",
			cr:         withSchemaName(konfiguration("giantswarm", "giantswarm", "giantswarm", "giantswarm"), "a/b"),
			privileged: privileged,
			wantFields: []string{".spec.targets.schema.reference.name"},
		},
		{
			name:       "empty namespaces are rejected",
			cr:         konfiguration("giantswarm", "", "giantswarm", ""),
			privileged: privileged,
			wantFields: []string{".spec.sources.flux.gitRepository.namespace", ".spec.destination.namespace"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckReconcilable(tc.cr, tc.privileged)

			switch {
			case tc.wantNamespaceErr:
				var namespaceErr *NamespaceNotAllowedError
				if !errors.As(err, &namespaceErr) {
					t.Fatalf("expected a NamespaceNotAllowedError, got: %v", err)
				}
				if !strings.Contains(err.Error(), tc.cr.Namespace) {
					t.Errorf("error %q does not name namespace %s", err.Error(), tc.cr.Namespace)
				}
			case tc.wantFields != nil:
				var referenceErr *InvalidReferenceError
				if !errors.As(err, &referenceErr) {
					t.Fatalf("expected an InvalidReferenceError, got: %v", err)
				}
				for _, field := range tc.wantFields {
					if !strings.Contains(err.Error(), field) {
						t.Errorf("error %q does not name %s", err.Error(), field)
					}
				}
			default:
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestInvalidValuesAreTruncated(t *testing.T) {
	cr := withGitRepositoryName(konfiguration("giantswarm", "giantswarm", "giantswarm", "giantswarm"), strings.Repeat("A", 40000))

	err := CheckReconcilable(cr, []string{"giantswarm"})
	if err == nil {
		t.Fatal("expected an error")
	}
	if len(err.Error()) > 1000 {
		t.Fatalf("error message is %d characters long, want it truncated", len(err.Error()))
	}
}

func TestParsePrivilegedNamespaces(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{name: "empty", input: "", expected: nil},
		{name: "single", input: "giantswarm", expected: []string{"giantswarm"}},
		{name: "default", input: "giantswarm,flux-giantswarm", expected: []string{"giantswarm", "flux-giantswarm"}},
		{name: "whitespace and empty entries", input: " giantswarm , ,flux-giantswarm,", expected: []string{"giantswarm", "flux-giantswarm"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParsePrivilegedNamespaces(tc.input)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Fatalf("ParsePrivilegedNamespaces(%q) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}

func TestValidateNamespaceList(t *testing.T) {
	testCases := []struct {
		name       string
		namespaces []string
		wantErr    bool
	}{
		{name: "empty", namespaces: nil},
		{name: "valid", namespaces: []string{"giantswarm", "flux-giantswarm"}},
		{name: "upper case", namespaces: []string{"GiantSwarm"}, wantErr: true},
		{name: "wildcard", namespaces: []string{"*"}, wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateNamespaceList(tc.namespaces)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateNamespaceList(%v) = %v, wantErr %t", tc.namespaces, err, tc.wantErr)
			}
		})
	}
}
