package konfigure

import (
	"context"
	"path"

	konfigureService "github.com/giantswarm/konfigure/pkg/service"
	konfigureVaultClient "github.com/giantswarm/konfigure/pkg/vaultclient"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func InitializeService(ctx context.Context, cacheDir, sopsKeysDir, installation string) (*konfigureService.Service, error) {
	// TODO It would be nice to be able to create the service vaultless
	vaultClient, err := konfigureVaultClient.NewClientUsingEnv(ctx)

	if err != nil {
		return nil, err
	}

	service, err := konfigureService.New(konfigureService.Config{
		VaultClient: vaultClient,

		Log:            log.FromContext(ctx),
		Dir:            path.Join(cacheDir, "latest"),
		Installation:   installation,
		SOPSKeysDir:    sopsKeysDir,
		SOPSKeysSource: "local",
		// TODO Does it make sense to be able to toggle this?
		Verbose: false,
	})

	if err != nil {
		return nil, err
	}

	return service, nil
}
