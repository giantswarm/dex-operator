package github

import (
	"github.com/giantswarm/microerror"
	"gopkg.in/yaml.v2"

	"github.com/giantswarm/dex-operator/pkg/dex/connectors"
)

func getSecretFromConfig(config string) (string, string, error) {
	if config == "" {
		return "", "", nil
	}
	configData := []byte(config)
	connectorConfig := &connectors.GitHubConfig{}
	if err := yaml.Unmarshal(configData, connectorConfig); err != nil {
		return "", "", microerror.Mask(err)
	}
	return connectorConfig.ClientID, connectorConfig.ClientSecret, nil
}
