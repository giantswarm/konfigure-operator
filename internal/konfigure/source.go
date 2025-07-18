package konfigure

import (
	"os"
	"path"
	"strings"

	"github.com/giantswarm/konfigure/pkg/fluxupdater"
)

func InitializeFluxUpdater(cacheDir, gitRepositoryNamespace, gitRepositoryName string) (*fluxupdater.FluxUpdater, error) {
	gitRepositoryAwareCacheDir := path.Join(cacheDir, gitRepositoryNamespace, gitRepositoryName)

	err := os.MkdirAll(gitRepositoryAwareCacheDir, 0750)
	if err != nil {
		return nil, err
	}

	updater, err := fluxupdater.New(fluxupdater.Config{
		CacheDir:      gitRepositoryAwareCacheDir,
		ApiServerHost: os.Getenv("KUBERNETES_SERVICE_HOST"),
		ApiServerPort: os.Getenv("KUBERNETES_SERVICE_PORT"),
		GitRepository: strings.Join([]string{gitRepositoryNamespace, gitRepositoryName}, "/"),
	})

	if err != nil {
		return nil, err
	}

	return updater, nil
}

func GetLastArchiveSHA(cacheDir string) (string, error) {
	bytes, err := os.ReadFile(path.Join(path.Clean(cacheDir), "lastarchive"))
	if err != nil {
		return "", err
	}

	content := string(bytes)
	parts := strings.Split(content, ".")

	return parts[0], nil
}
