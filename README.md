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

### azure active directory

Configures app registration in an azure active directory tenant.
To configure this identity provider, first [add two app registrations to the tenant](https://learn.microsoft.com/en-us/azure/active-directory/develop/quickstart-register-app).

- Name: `giantswarm-dex`. It needs `delegated` Permissions `Directory.Read.All` and `User.Read`.
- Name: `dex-operator`. It needs `Application` Permissions `Application.ReadWrite.All` and a client secret configured.


Then, add the following configuration to `values`:
```
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
- `$CLIENTID`: ID of the client (application) configured in the tenant for `dex-operator`.
- `$CLIENTSECRET`: Secret configured for `dex-operator` client.

When the configuration is added, a `microsoft` connector should be added to each installed `dex-app` and the application registration with callback URI should be visible in the active directory.
