package logic

import (
	"fmt"
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestShouldReconcile(t *testing.T) {
	testCases := []struct {
		name     string
		input    v1.ObjectMeta
		expected bool
	}{
		{
			name:     "no labels",
			input:    v1.ObjectMeta{},
			expected: true,
		},
		{
			name: "no reconcile label",
			input: v1.ObjectMeta{
				Labels: map[string]string{
					GeneratedByLabel:     "konfigure-operator",
					OwnerApiGroupLabel:   "konfigure.giantswarm.io",
					OwnerApiVersionLabel: "v1alpha1",
					OwnerKindLabel:       "ManagementClusterConfiguration",
					OwnerNameLabel:       "collection-konfiguration",
					OwnerNamespaceLabel:  "giantswarm",
					RevisionLabel:        "868c6981ac65c7178da5c10470f9ada21963a7e3",
				},
			},
			expected: true,
		},
		{
			name: "reconcile label present with value enabled",
			input: v1.ObjectMeta{
				Labels: map[string]string{
					"foo":          "bar",
					ReconcileLabel: EnabledValue,
				},
			},
			expected: true,
		},
		{
			name: "reconcile label present with random value",
			input: v1.ObjectMeta{
				Labels: map[string]string{
					"foo":          "bar",
					ReconcileLabel: "anything",
				},
			},
			expected: true,
		},
		{
			name: "reconcile label present with value disabled",
			input: v1.ObjectMeta{
				Labels: map[string]string{
					"foo":          "bar",
					ReconcileLabel: DisabledValue,
				},
			},
			expected: false,
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d: %s", i, tc.name), func(t *testing.T) {
			result := ShouldReconcile(tc.input)

			if result != tc.expected {
				t.Fatalf("result does not match, expected: %v, got: %v", tc.expected, result)
			}
		})
	}
}
