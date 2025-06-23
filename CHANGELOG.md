# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.13.0] - 2025-06-23

### Changed

- Change connector display names from "Azure AD" to "Entry ID"
- Resolve updated code linter findings.

## [0.12.4] - 2025-03-05

### Fixed

- fix issue with image registry parsing

## [0.12.3] - 2025-03-05

### Added

- Add support for hostAliases on Chart level

### Changed

- Change ownership to Team Shield

### Fixed

- Disable zap logger development mode to avoid panicking

## [0.12.2] - 2024-07-25

### Fixed

- Fixes critical CVEs in dex library in https://github.com/giantswarm/dex-operator/pull/110

## [0.12.1] - 2023-12-06

### Changed

- Configure `gsoci.azurecr.io` as the default container image registry.

## [0.12.0] - 2023-12-05

### Added

- Add customer write-all groups to auth-configmap

### Removed

- Remove secret migration code

## [0.11.0] - 2023-11-28

### Added

- Add `simple` provider for default access.

## [0.10.1] - 2023-10-24

### Fixed

- Look for Cluster in organization namespace in vintage MCs

### Added

- Add option to `setup` to use credentials as base64 encoded variables.

## [0.10.0] - 2023-10-02

### Changed

- Propagate `global.podSecurityStandards.enforced` value set to `false` for PSS migration

## [0.9.0] - 2023-09-14

### Changed

- Changed GitHub connector config to include `teamNameField: slug` by default.

## [0.8.0] - 2023-08-02

### Added

- Add api server port to auth configmap

## [0.7.0] - 2023-07-12

## [0.6.0] - 2023-07-11

### Added

- Create auth configmap containing MC name and admin group info

## [0.5.2] - 2023-07-04

### Changed

- Update deployment to be PSS compliant.

### Added

- Add icon to Chart.yaml.

## [0.5.1] - 2023-06-02

### Added

- Add team label.

## [0.5.0] - 2023-05-31

### Added

- Add functionality to delete all app registrations for an installation to providers.

## [0.4.0] - 2023-05-23

### Added

- Use dex app name specific naming pattern for dex config secret and add migration code for old naming pattern.

## [0.3.6] - 2023-05-17

### Added

- Add predicate to app controller that recognizes MC dex app even before the `app.kubernetes.io/name` label was set.

## [0.3.5] - 2023-05-10

### Changed

- Allow user configmaps for dex app as long as no connectors are defined.

### Removed

- Stop pushing to `openstack-app-collection`.

## [0.3.4] - 2023-04-04

### Fixed

- Fix bug in azure provider when update of redirect URI array fails.

## [0.3.3] - 2023-03-29

### Fixed

- Fix bug where credential creation fails when parent directory is missing.
- Omit empty fields in credential creation.

### Changed

- Replace credential creation script with `opsctl create dexconfig`.

## [0.3.2] - 2023-03-23

### Changed

- Push to vsphere and cloud-director app collection.
- Don't push to openstack app collection.
- Move issuer address computation into its own function.

## [0.3.1] - 2023-03-22

### Changed

- Make dex operator modules exportable

## [0.3.0] - 2023-03-20

### Added

- Add credential creation, update and cleanup functionality to github provider
- Add `setup` package with functionality to create dex-operator credentials
- Add credential creation, update and cleanup functionality to azure provider
- Add github provider

## [0.2.0] - 2023-02-27

### Added

- Add use of runtime/default seccomp profile.

### Changed

- Rotate azure credentials 10 days before expiry
- Added a new config parameter for issuer address
- Adjusted creation of callback URL to prefer issuer address over base domain if possible
- Allowed more volumes in the PSP to sync with restricted.

## [0.1.4] - 2022-12-21

### Added

- Add prometheus metrics about dex-app registrations on identity providers

### Changed

- Do not reconcile dex apps that have a user configmap specified and remove configuration in that case. This is to prevent a bug where connectors can be overwritten. 
- Omit empty lists of connectors from marshalled OIDC owner data

## [0.1.3] - 2022-12-07

### Changed

- Push to CAPI provider app collections
- Allow empty list of providers
- Raise memory usage limit to 500Mi

## [0.1.2] - 2022-12-06

### Changed

- Push to kvm app collection.
- Push to aliyun

## [0.1.1] - 2022-12-06

### Changed

- Improve `README.md`
- Push to aws and azure app collections.

## [0.1.0] - 2021-12-15

### Added

- Added the script that was used to create dex-operator credentials.
- Added update logic and key rotation for identity providers.
- Added finalizer management for default dex config secret.
- Added `README.md`.
- Added azure active directory provider.
- Added initial implementation of the dex operator.

[Unreleased]: https://github.com/giantswarm/dex-operator/compare/v0.13.0...HEAD
[0.13.0]: https://github.com/giantswarm/dex-operator/compare/v0.12.4...v0.13.0
[0.12.4]: https://github.com/giantswarm/dex-operator/compare/v0.12.3...v0.12.4
[0.12.3]: https://github.com/giantswarm/dex-operator/compare/v0.12.2...v0.12.3
[0.12.2]: https://github.com/giantswarm/dex-operator/compare/v0.12.1...v0.12.2
[0.12.1]: https://github.com/giantswarm/dex-operator/compare/v0.12.0...v0.12.1
[0.12.0]: https://github.com/giantswarm/dex-operator/compare/v0.11.0...v0.12.0
[0.11.0]: https://github.com/giantswarm/dex-operator/compare/v0.10.1...v0.11.0
[0.10.1]: https://github.com/giantswarm/dex-operator/compare/v0.10.0...v0.10.1
[0.10.0]: https://github.com/giantswarm/dex-operator/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/giantswarm/dex-operator/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/giantswarm/dex-operator/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/giantswarm/dex-operator/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/giantswarm/dex-operator/compare/v0.5.2...v0.6.0
[0.5.2]: https://github.com/giantswarm/dex-operator/compare/v0.5.1...v0.5.2
[0.5.1]: https://github.com/giantswarm/dex-operator/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/giantswarm/dex-operator/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/giantswarm/dex-operator/compare/v0.3.6...v0.4.0
[0.3.6]: https://github.com/giantswarm/dex-operator/compare/v0.3.5...v0.3.6
[0.3.5]: https://github.com/giantswarm/dex-operator/compare/v0.3.4...v0.3.5
[0.3.4]: https://github.com/giantswarm/dex-operator/compare/v0.3.3...v0.3.4
[0.3.3]: https://github.com/giantswarm/dex-operator/compare/v0.3.2...v0.3.3
[0.3.2]: https://github.com/giantswarm/dex-operator/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/giantswarm/dex-operator/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/giantswarm/dex-operator/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/giantswarm/dex-operator/compare/v0.1.4...v0.2.0
[0.1.4]: https://github.com/giantswarm/dex-operator/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/giantswarm/dex-operator/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/giantswarm/dex-operator/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/giantswarm/dex-operator/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/giantswarm/dex-operator/releases/tag/v0.1.0
