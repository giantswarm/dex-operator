# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/giantswarm/dex-operator/compare/v0.1.4...HEAD
[0.1.4]: https://github.com/giantswarm/dex-operator/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/giantswarm/dex-operator/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/giantswarm/dex-operator/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/giantswarm/dex-operator/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/giantswarm/dex-operator/releases/tag/v0.1.0
