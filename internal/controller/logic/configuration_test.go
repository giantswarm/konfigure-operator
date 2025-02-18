package logic

import (
	"fmt"
	"reflect"
	"regexp/syntax"
	"testing"
)

func TestFilterApps(t *testing.T) {
	testCases := []struct {
		name                 string
		allApps              []string
		includeExactMatchers []string
		includeRegexMatchers []string
		excludeExactMatchers []string
		excludeRegexMatchers []string
		expectedMatch        []string
		expectedMiss         []string
		expectedError        error
	}{
		{
			name:                 "no matchers should return all apps",
			allApps:              []string{"a", "b", "c"},
			includeExactMatchers: []string{},
			includeRegexMatchers: []string{},
			expectedMatch:        []string{"a", "b", "c"},
			expectedMiss:         []string{},
		},
		{
			name:                 "no apps, no results",
			allApps:              nil,
			includeExactMatchers: nil,
			includeRegexMatchers: nil,
			expectedMatch:        []string{},
			expectedMiss:         []string{},
		},
		{
			name:                 "handle nil matchers, should return all apps",
			allApps:              []string{"x", "y", "z"},
			includeExactMatchers: nil,
			includeRegexMatchers: nil,
			expectedMatch:        []string{"x", "y", "z"},
			expectedMiss:         []string{},
		},
		{
			name:                 "results are returned ordered",
			allApps:              []string{"b", "d", "a", "c"},
			includeExactMatchers: []string{"y", "c", "a", "x"},
			includeRegexMatchers: []string{},
			expectedMatch:        []string{"a", "c"},
			expectedMiss:         []string{"x", "y"},
		},
		{
			name:                 "valid regex matchers",
			allApps:              []string{"app-operator", "trivy", "observability-bundle", "trivy-operator", "operator-zero"},
			includeExactMatchers: []string{},
			includeRegexMatchers: []string{"trivy.*", ".*-operator"},
			expectedMatch:        []string{"app-operator", "trivy", "trivy-operator"},
			expectedMiss:         []string{},
		},
		{
			name:                 "using group matcher",
			allApps:              []string{"chart-operator", "app-exporter", "observability-bundle", "app-asd-qwe", "app-operator", "chart-app-controller"},
			includeExactMatchers: []string{},
			includeRegexMatchers: []string{"^app-([a-zA-Z]+)$"},
			expectedMatch:        []string{"app-exporter", "app-operator"},
			expectedMiss:         []string{},
			expectedError:        &syntax.Error{},
		},
		{
			name:                 "invalid regular expression poisons the whole filter",
			allApps:              []string{"apple", "pear", "blueberry"},
			includeExactMatchers: []string{},
			includeRegexMatchers: []string{".*p.*", "$^*.a-z\\"},
			expectedMatch:        []string{},
			expectedMiss:         []string{},
			expectedError:        &syntax.Error{},
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d: %s", i, tc.name), func(t *testing.T) {
			matches, misses, err := filterApps(tc.allApps, tc.includeExactMatchers, tc.includeRegexMatchers, tc.excludeExactMatchers, tc.excludeRegexMatchers)
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
