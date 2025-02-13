package logic

import (
	"fmt"
	"reflect"
	"regexp/syntax"
	"testing"
)

func TestFilterApps(t *testing.T) {
	testCases := []struct {
		name          string
		allApps       []string
		exactMatchers []string
		regexMatchers []string
		expectedMatch []string
		expectedMiss  []string
		expectedError error
	}{
		{
			name:          "no matchers should return all apps",
			allApps:       []string{"a", "b", "c"},
			exactMatchers: []string{},
			regexMatchers: []string{},
			expectedMatch: []string{"a", "b", "c"},
			expectedMiss:  []string{},
		},
		{
			name:          "no apps, no results",
			allApps:       nil,
			exactMatchers: nil,
			regexMatchers: nil,
			expectedMatch: []string{},
			expectedMiss:  []string{},
		},
		{
			name:          "handle nil matchers, should return all apps",
			allApps:       []string{"x", "y", "z"},
			exactMatchers: nil,
			regexMatchers: nil,
			expectedMatch: []string{"x", "y", "z"},
			expectedMiss:  []string{},
		},
		{
			name:          "results are returned ordered",
			allApps:       []string{"b", "d", "a", "c"},
			exactMatchers: []string{"y", "c", "a", "x"},
			regexMatchers: []string{},
			expectedMatch: []string{"a", "c"},
			expectedMiss:  []string{"x", "y"},
		},
		{
			name:          "valid regex matchers",
			allApps:       []string{"app-operator", "trivy", "observability-bundle", "trivy-operator", "operator-zero"},
			exactMatchers: []string{},
			regexMatchers: []string{"trivy.*", ".*-operator"},
			expectedMatch: []string{"app-operator", "trivy", "trivy-operator"},
			expectedMiss:  []string{},
		},
		{
			name:          "using group matcher",
			allApps:       []string{"chart-operator", "app-exporter", "observability-bundle", "app-asd-qwe", "app-operator", "chart-app-controller"},
			exactMatchers: []string{},
			regexMatchers: []string{"^app-([a-zA-Z]+)$"},
			expectedMatch: []string{"app-exporter", "app-operator"},
			expectedMiss:  []string{},
			expectedError: &syntax.Error{},
		},
		{
			name:          "invalid regular expression poisons the whole filter",
			allApps:       []string{"apple", "pear", "blueberry"},
			exactMatchers: []string{},
			regexMatchers: []string{".*p.*", "$^*.a-z\\"},
			expectedMatch: []string{},
			expectedMiss:  []string{},
			expectedError: &syntax.Error{},
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d: %s", i, tc.name), func(t *testing.T) {
			matches, misses, err := filterApps(tc.allApps, tc.exactMatchers, tc.regexMatchers)
			if err != nil {
				if tc.expectedError != nil {
					if !(reflect.TypeOf(err) == reflect.TypeOf(tc.expectedError)) {
						t.Fatalf("expected error: %v, got: %v", tc.expectedError, err)
					}
				} else {
					t.Fatalf("unexpected error: %v", err)
				}
			}

			if !reflect.DeepEqual(matches, tc.expectedMatch) {
				t.Fatalf("expected apps to matches: %v, got: %v", tc.expectedMatch, matches)
			}

			if !reflect.DeepEqual(misses, tc.expectedMiss) {
				t.Fatalf("expected apps to misses: %v, got: %v", tc.expectedMiss, misses)
			}
		})
	}
}
