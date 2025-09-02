package controller

import (
	"context"
	"os"
	"path"

	"github.com/giantswarm/konfigure/v2/pkg/fluxupdater"
	"github.com/giantswarm/konfigure/v2/pkg/sopsenv"

	konfigurev1alpha1 "github.com/giantswarm/konfigure-operator/api/v1alpha1"
	"github.com/giantswarm/konfigure-operator/internal/konfigure"
)

func InitializeSopsEnv(ctx context.Context, dir string) (*sopsenv.SOPSEnv, error) {
	err := os.MkdirAll(path.Clean(dir), 0700)
	if err != nil {
		return nil, err
	}

	sopsEnv, err := konfigure.InitializeSopsEnvFromKubernetes(ctx, dir)

	if err != nil {
		return nil, err
	}

	err = sopsEnv.Setup(ctx)
	if err != nil {
		return sopsEnv, err
	}

	return sopsEnv, nil
}

func InitializeFluxUpdater(dir string, fluxSource konfigurev1alpha1.FluxSource) (*fluxupdater.FluxUpdater, error) {
	err := os.MkdirAll(path.Clean(dir), 0700)
	if err != nil {
		return nil, err
	}

	fluxUpdater, err := konfigure.InitializeFluxUpdater(dir, fluxSource.GitRepository.Namespace, fluxSource.GitRepository.Name)

	if err != nil {
		return nil, err
	}

	err = fluxUpdater.UpdateConfig()

	if err != nil {
		return fluxUpdater, err
	}

	return fluxUpdater, nil
}
