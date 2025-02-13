package konfigure

import (
	"os"
	"path"
	"strings"

	"github.com/giantswarm/konfigure/pkg/fluxupdater"
)

func InitializeFluxUpdater(cacheDir, sourceControllerService, gitRepositoryNamespace, gitRepositoryName string) (*fluxupdater.FluxUpdater, error) {
	gitRepositoryAwareCacheDir := path.Join(cacheDir, gitRepositoryNamespace, gitRepositoryName)

	err := os.MkdirAll(gitRepositoryAwareCacheDir, 0755)
	if err != nil {
		return nil, err
	}

	updater, err := fluxupdater.New(fluxupdater.Config{
		CacheDir:                gitRepositoryAwareCacheDir,
		ApiServerHost:           os.Getenv("KUBERNETES_SERVICE_HOST"),
		ApiServerPort:           os.Getenv("KUBERNETES_SERVICE_PORT"),
		SourceControllerService: sourceControllerService,
		GitRepository:           strings.Join([]string{gitRepositoryNamespace, gitRepositoryName}, "/"),
	})

	if err != nil {
		return nil, err
	}

	return updater, nil
}
