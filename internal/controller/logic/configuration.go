package logic

import (
	"regexp"
	"slices"

	mapset "github.com/deckarep/golang-set/v2"

	konfigurev1alpha1 "github.com/giantswarm/konfigure-operator/api/v1alpha1"
	"github.com/giantswarm/konfigure-operator/internal/ccr"
)

func GetAppsToReconcile(dir string, cr *konfigurev1alpha1.Configuration) (match []string, miss []string, err error) {
	ccr, err := ccr.New(dir)
	if err != nil {
		return []string{}, []string{}, err
	}

	all, err := ccr.ListApps()
	if err != nil {
		return []string{}, []string{}, err
	}

	return filterApps(all, cr.Applications.Includes.ExactMatchers, cr.Applications.Includes.RegexMatchers, cr.Applications.Excludes.ExactMatchers, cr.Applications.Excludes.RegexMatchers)
}

// filterApps Filter apps that match any of the exact or regex matchers and also return
// a list of exact matcher that was requested but not found a match for them.
func filterApps(all, includeExactMatchers, includeRegexMatchers, excludeExactMatchers, excludeRegexMatcher []string) (match []string, miss []string, err error) {
	if all == nil {
		return []string{}, []string{}, nil
	}

	matchSet := mapset.NewSet[string]()
	missSet := mapset.NewSet[string]()

	allSet := mapset.NewSet[string]()
	for _, app := range all {
		allSet.Add(app)
	}

	// Includes
	if len(includeExactMatchers) == 0 && len(includeRegexMatchers) == 0 {
		matchSet = matchSet.Union(allSet)
	} else {
		for _, app := range includeExactMatchers {
			if allSet.Contains(app) {
				matchSet.Add(app)
			} else {
				missSet.Add(app)
			}
		}

		for _, expression := range includeRegexMatchers {
			compiled, err := regexp.Compile(expression)
			if err != nil {
				return []string{}, []string{}, err
			}

			for _, app := range all {
				if compiled.MatchString(app) {
					matchSet.Add(app)
				}
			}
		}
	}

	// Excludes
	for _, app := range excludeExactMatchers {
		if matchSet.Contains(app) {
			matchSet.Remove(app)
		}

		if missSet.Contains(app) {
			missSet.Remove(app)
		}
	}

	for _, expression := range excludeRegexMatcher {
		compiled, err := regexp.Compile(expression)
		if err != nil {
			return []string{}, []string{}, err
		}

		for _, app := range matchSet.ToSlice() {
			if compiled.MatchString(app) {
				matchSet.Remove(app)
			}
		}

		for _, app := range missSet.ToSlice() {
			if compiled.MatchString(app) {
				missSet.Remove(app)
			}
		}
	}

	matches := matchSet.ToSlice()
	misses := missSet.ToSlice()

	slices.Sort(matches)
	slices.Sort(misses)

	return matches, misses, nil
}
