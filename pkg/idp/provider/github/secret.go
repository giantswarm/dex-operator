package github

import (
	githubconnector "github.com/dexidp/dex/connector/github"
	"github.com/giantswarm/microerror"
	"gopkg.in/yaml.v2"
)

func getSecretFromConfig(config string) (string, error) {
	if config == "" {
		return "", nil
	}
	configData := []byte(config)
	connectorConfig := &githubconnector.Config{}
	if err := yaml.Unmarshal(configData, connectorConfig); err != nil {
		return "", microerror.Mask(err)
	}
	return connectorConfig.ClientSecret, nil
}
