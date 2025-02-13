package konfigure

import (
	"context"

	"github.com/giantswarm/konfigure/pkg/sopsenv"
	sopsenvKey "github.com/giantswarm/konfigure/pkg/sopsenv/key"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func InitializeSopsEnvFromKubernetes(ctx context.Context, keysDir string) (*sopsenv.SOPSEnv, error) {
	config := sopsenv.SOPSEnvConfig{
		KeysDir:    keysDir,
		KeysSource: sopsenvKey.KeysSourceKubernetes,
		Logger:     log.FromContext(ctx),
	}

	sopsEnv, err := sopsenv.NewSOPSEnv(config)

	if err != nil {
		return nil, err
	}

	return sopsEnv, nil
}
