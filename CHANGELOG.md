# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/giantswarm/dex-operator/compare/v0.3.3...HEAD
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
