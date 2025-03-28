package logic

import (
	"fmt"
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
)

func TestFilterExternalFromMap(t *testing.T) {
	testCases := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name:     "nil should result in empty map",
			input:    nil,
			expected: map[string]string{},
		},
		{
			name:     "empty configmap annotation should result in empty map",
			input:    v1.ConfigMap{}.Annotations,
			expected: map[string]string{},
		},
		{
			name:     "empty configmap labels should result in empty map",
			input:    v1.ConfigMap{}.Annotations,
			expected: map[string]string{},
		},
		{
			name:     "empty secret annotation should result in empty map",
			input:    v1.Secret{}.Annotations,
			expected: map[string]string{},
		},
		{
			name:     "empty secret labels should result in empty map",
			input:    v1.Secret{}.Annotations,
			expected: map[string]string{},
		},
		{
			name: "filter out all of it",
			input: map[string]string{
				KonfigureOperatorPrefix + "/example-1": "test-1",
				KonfigureOperatorPrefix + "/example-2": "test-2",
				KonfigureOperatorPrefix + "/example-3": "test-3",
			},
			expected: map[string]string{},
		},
		{
			name: "filter out all of it",
			input: map[string]string{
				KonfigureOperatorPrefix + "/example-1": "test-1",
				KonfigureOperatorPrefix + "/example-2": "test-2",
				KonfigureOperatorPrefix + "/example-3": "test-3",
			},
			expected: map[string]string{},
		},
		{
			name: "all external, same result",
			input: map[string]string{
				"example-1": "test-1",
				"example-2": "test-2",
				"example-3": "test-3",
			},
			expected: map[string]string{
				"example-1": "test-1",
				"example-2": "test-2",
				"example-3": "test-3",
			},
		}, {
			name: "correct filtering",
			input: map[string]string{
				"hello-3":                              "world1-3",
				"hello-2":                              "world1-2",
				KonfigureOperatorPrefix + "/example-1": "test-1",
				"hello-1":                              "world1-1",
				KonfigureOperatorPrefix + "/example-2": "test-2",
				KonfigureOperatorPrefix + "/example-3": "test-3",
			},
			expected: map[string]string{
				"hello-1": "world1-1",
				"hello-2": "world1-2",
				"hello-3": "world1-3",
			},
		}, {
			name: "filter is based on prefix of the key, considered external otherwise",
			input: map[string]string{
				"example-2": "test-2",
				"hello-" + KonfigureOperatorPrefix + "/example-1": "test-1",
				"example-3":                                  "test-3",
				KonfigureOperatorPrefix + "/example-2":       "test-2",
				" " + KonfigureOperatorPrefix + "/example-3": "test-3",
				"example-1":                                  KonfigureOperatorPrefix,
			},
			expected: map[string]string{
				" " + KonfigureOperatorPrefix + "/example-3": "test-3",
				"example-1": KonfigureOperatorPrefix,
				"example-2": "test-2",
				"example-3": "test-3",
				"hello-" + KonfigureOperatorPrefix + "/example-1": "test-1",
			},
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d: %s", i, tc.name), func(t *testing.T) {
			result := FilterExternalFromMap(tc.input)

			if result == nil {
				t.Fatalf("result should not be nil")
			}

			if len(result) != len(tc.expected) {
				t.Fatalf("result should have length %d, but has length %d", len(tc.expected), len(result))
			}

			if !reflect.DeepEqual(result, tc.expected) {
				t.Fatalf("expected result: %v, got: %v", tc.expected, result)
			}

			result["test-addition"] = "test"
		})
	}
}
