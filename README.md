# dex-operator

An operator that automates the management of dex connector configurations

## functionality

The `dex-operator` can be deployed to giant swarm management clusters.
The `app-controller` then reconciles all `application.giantswarm.io` custom resources for the [dex-app](https://github.com/giantswarm/dex-app) which are present on the management cluster.
This also includes `dex-app` instances deployed to workload clusters.
`dex-operator` automates management of identity provider app registrations for these `dex` instances.
To do this it can be configured with a list of identity provider credentials to set up applications in.
The `app controller` configures callback URIs and other settings and writes the resulting `connectors` back into the `dex-app` instances configuration.

## providers

Providers need to implement the `provider.Provider` interface.
Currently supported providers are `azure active directory` and `github`.
In addition, the `simple` provider offers a basic way to include any identity provider [supported by dex](https://dexidp.io/docs/connectors/).

### adding dex-operator credentials for gs installations

Each `dex-operator` instance should have its own credentials and client registration for each organization/tenant.
[opsctl](https://github.com/giantswarm/opsctl) supports the creation, update and cleanup of credentials for `dex-operator` via the `create dexconfig` command.

### Azure Active Directory

Configures app registration in an azure active directory tenant.
`dex-operator` will look for [two app registrations](https://learn.microsoft.com/en-us/azure/active-directory/develop/quickstart-register-app) on the tenant.

- Name: `giantswarm-dex`. It needs `delegated` Permissions `Directory.Read.All` and `User.Read`. Delegated permissions set here will cascade to dex apps registered by the dex-operator.
- Name: `dex-operator`. It needs `Application` Permissions `Application.ReadWrite.All`. Permissions set here will cascade to dex-operator instances registered by the user.


The configuration for Azure Active Directory in  `values` looks like this:
```yaml
oidc:
  $OWNER:
    providers:
    - name: ad
      credentials:
        client-id: $CLIENTID
        client-secret: $CLIENTSECRET
        tenant-id: $TENANTID
```
- `$OWNER`: Owner of the azure tenant. `giantswarm` or `customer`.
- `$TENANTID`: The ID of the azure tenant that should be used for the configuration.
- `$CLIENTID`: ID of the client (application) configured in the tenant for `dex-operator` client for the management cluster `dex-operator` runs on.
- `$CLIENTSECRET`: Secret configured for `dex-operator` client for the management cluster `dex-operator` runs on.

When the configuration is present, a `microsoft` connector will be added to each installed `dex-app` and the application registration with callback URI should be visible in the active directory.
The operator will automatically renew the client-secret in case it expires or is removed from a connector.
It will also automatically update other configuration such as permissions, claims and redirect URI.

### GitHub

Configures app registration in a GitHub organization.
Please note that the github provider has limited functionality due to limitations in the GitHub API.
We recommend GitHub to be configured as a fallback SSO method.


The configuration for GitHub in  `values` looks like this:
```yaml
oidc:
  $OWNER:
    providers:
    - name: github
      credentials:
        client-id: $CLIENTID
        client-secret: $CLIENTSECRET
        organization: $ORGANIZATION
        team: $TEAM
        app-id: $APPID
        private-key: $PRIVATEKEY
```
- `$OWNER`: Owner of the github organization. `giantswarm` or `customer`.
- `$ORGANIZATION`: The name of the github organization that should be used for the configuration.
- `$TEAM`: The name of the github team that should be used for SSO.
- `$CLIENTID`: Client ID for the github app in the organization for the management cluster `dex-operator` runs on which should be used for SSO.
- `$CLIENTSECRET`: Client Secret for the github app in the organization for the management cluster `dex-operator` runs on which should be used for SSO.
- `$APPID`: ID of the github app in the organization for the management cluster `dex-operator` runs on which should be used for API calls.
- `$PRIVATEKEY`: Private key for the github app in the organization for the management cluster `dex-operator` runs on which should be used for API calls.


When the configuration is present, a `github` connector will be added to each installed `dex-app`.
The GitHub API does not allow to automatically update apps or renew the client-secrets.
Unfortunately it also does not allow for access to workload cluster callback URLs.
However, it will provide metrics that allow alerting when rotation is needed.
In that case [opsctl](https://github.com/giantswarm/opsctl) supports the update via the `create dexconfig --provider github --update` command.
The `--workload-cluster` flag also allows creation of callback URLs for up to 9 workload clusters.

### Simple Provider

The simple provider does not implement a client and therefore does not communicate with identity providers or create new configuration.
It can merely distribute existing connector configuration from the management cluster across workload cluster dex instances.
This allows users with management cluster access a default access method without further configuration.
It also allows dex-operator to work without needing permissions on an identity provider or without needing to support it explicitly.
However, __we strongly recommend using different connectors for each workload cluster and automatic secret rotation__, either manually or through providers like [`azure active directory`](#azure-active-directory)

Reusing configuration across clusters is always a security risk since leaking one secret can compromise several organizations.

The configuration for the simple provider in  `values` looks like this:
```yaml
oidc:
  $OWNER:
    providers:
    - name: simple
      credentials:
        connectorType: $CONNECTORTYPE
        connectorConfig: $CONNECTORCONFIG
```

- `$OWNER`: Owner of the connector configuration. `giantswarm` or `customer`.
- `$CONNECTORTYPE`: The type of dex connector. All valid types can be found in the [dex documentation](https://dexidp.io/docs/connectors/).
- `$CONNECTORCONFIG`: The connector configuration. Format for each types can likewise be found in the [dex documentation](https://dexidp.io/docs/connectors/). Note that `redirectURI` is not needed since it will be injected for each dex instance.

### Host Aliases

You can define custom host aliases for the dex-operator pod by setting the `hostAliases` parameter:

```yaml
hostAliases:
  - ip: "192.168.1.1"
    hostnames:
      - "github.com"
...
```
