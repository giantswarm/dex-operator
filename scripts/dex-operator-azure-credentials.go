// TODO this script should be replaced by a general tool with flags, this doesn't make much sense but is the leftover from the original script and it does the job

/*
In order to manage dex applications in azure, each dex-operator will need it's own credential secret.
Creating a new credential can be done manually via azure portal. However, the following script can be used as well.

The following environment variables need to be set:

INSTALLATION - use installation name of the installation the credentials should be used for.
CREDENTIAL_FILE - location of config secret patch file containing credentials currently used by dex-operator. Can be for the same installation (to renew credential secret) or for another one (create a new credential).
ACTION - "update" to create new credentials for dex-operator or "clean" to delete other credentials for the installation that are not in use anymore. (in this case, installation and given credential need to match!)

The output matches the giantswarm config format.

Example to run it for an installation:

export ACTION=update
export CREDENTIAL_FILE=tmp
export INSTALLATION=test
go run scripts/dex-operator-azure-credentials.go > $INSTALLATION

*/

package main

import (
	"fmt"
	"giantswarm/dex-operator/pkg/idp/provider/azure"
	"giantswarm/dex-operator/setup"
	"os"
)

const (
	InstallationKey = "INSTALLATION"
	CredentialKey   = "CREDENTIAL_FILE"
	ActionKey       = "ACTION"
)

func getConfig() (setup.SetupConfig, error) {
	installation := os.Getenv(InstallationKey)
	if installation == "" {
		return setup.SetupConfig{}, fmt.Errorf("%s must not be empty", InstallationKey)
	}
	credentialFile := os.Getenv(CredentialKey)
	if credentialFile == "" {
		return setup.SetupConfig{}, fmt.Errorf("%s must not be empty", CredentialKey)
	}
	action := os.Getenv(ActionKey)
	if action == "" {
		return setup.SetupConfig{}, fmt.Errorf("%s must not be empty", ActionKey)
	}

	return setup.SetupConfig{
		Installation:   installation,
		CredentialFile: credentialFile,
		Action:         action,
		Provider:       azure.ProviderName,
	}, nil
}

func main() {
	err := run()
	if err != nil {
		fmt.Printf("failed due to error %s", err)
		os.Exit(1)
	}
}

func run() error {
	command, err := getConfig()
	if err != nil {
		return err
	}
	setup, err := setup.New(command)
	if err != nil {
		return err
	}
	return setup.Run()
}
