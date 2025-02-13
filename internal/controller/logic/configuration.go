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

	return filterApps(all, cr.Applications.ExactMatchers, cr.Applications.RegexMatchers)
}

// filterApps Filter apps that match any of the exact or regex matchers and also return
// a list of exact matcher that was requested but not found a match for them.
func filterApps(all, exactMatchers, regexMatchers []string) (match []string, miss []string, err error) {
	if all == nil {
		return []string{}, []string{}, nil
	}

	if len(exactMatchers) == 0 && len(regexMatchers) == 0 {
		return all, []string{}, nil
	}

	allSet := mapset.NewSet[string]()
	for _, app := range all {
		allSet.Add(app)
	}

	matchSet := mapset.NewSet[string]()
	missSet := mapset.NewSet[string]()

	for _, app := range exactMatchers {
		if allSet.Contains(app) {
			matchSet.Add(app)
		} else {
			missSet.Add(app)
		}
	}

	for _, expression := range regexMatchers {
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

	matches := matchSet.ToSlice()
	misses := missSet.ToSlice()

	slices.Sort(matches)
	slices.Sort(misses)

	return matches, misses, nil
}
