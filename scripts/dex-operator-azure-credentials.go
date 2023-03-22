// TODO this script should be replaced by a general tool with flags, this doesn't make much sense but is the leftover from the original script and it does the job

/*
In order to manage dex applications in azure, each dex-operator will need it's own credentials.
Creating a new set of credentials can be done manually via azure portal. However, the following script can be used as well.
It calls the setup package.

The following environment variables need to be set:

INSTALLATION - use installation name of the installation the credentials should be used for.
INPUT_FILE - location of config secret patch file containing credentials currently used by dex-operator. Can be for the same installation (to update credential secret) or for another one (create a new credential).
ACTION - "create" to create a new set of credentials, "update" to roll azure credentials for dex-operator or "clean" to delete other azure credentials for the installation that are not in use anymore.

The output matches the giantswarm config format.

Example to run it for an installation:

export ACTION=update
export INPUT_FILE=tmp
export INSTALLATION=test
go run scripts/dex-operator-azure-credentials.go

For "create" and "update" actions:
In case a new service principal is created, a browser window will be opened in which admin consent for api permissions are requested.
The permissions need to be granted in order for dex-operator to run using the new credentials.
In case the service principal already exists, only a new secret will be created.

For the "clean" action:
After adding credentials with "update" or "create" command and dex-operator is now using the new credential, run the "clean" command to get rid of leftover secrets.
*/

package main

import (
	"fmt"
	"os"

	"github.com/giantswarm/dex-operator/pkg/idp/provider/azure"
	"github.com/giantswarm/dex-operator/setup"
)

const (
	InstallationKey = "INSTALLATION"
	FileKey         = "INPUT_FILE"
	ActionKey       = "ACTION"
)

func getConfig() (setup.SetupConfig, error) {
	installation := os.Getenv(InstallationKey)
	if installation == "" {
		return setup.SetupConfig{}, fmt.Errorf("%s must not be empty", InstallationKey)
	}
	credentialFile := os.Getenv(FileKey)
	if credentialFile == "" {
		return setup.SetupConfig{}, fmt.Errorf("%s must not be empty", FileKey)
	}
	action := os.Getenv(ActionKey)
	if action == "" {
		return setup.SetupConfig{}, fmt.Errorf("%s must not be empty", ActionKey)
	}

	return setup.SetupConfig{
		Installation:   installation,
		CredentialFile: credentialFile,
		OutputFile:     installation,
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
