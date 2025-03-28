package logic

import "strings"

func FilterExternalFromMap(existing map[string]string) map[string]string {
	externals := make(map[string]string)

	for key, value := range existing {
		if strings.HasPrefix(key, KonfigureOperatorPrefix) {
			continue
		}

		externals[key] = value
	}

	return externals
}
