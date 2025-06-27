# Chaqngelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.6.0] - 2025-06-27

## [0.5.1] - 2025-05-14

## [0.5.0] - 2025-04-30

### Added

- Add push releases to CAPx root collection repositories.

## [0.4.0] - 2025-04-30

### Changed

- Push to `control-plane-catalog` instead of `giantswarm`.

## [0.3.1] - 2025-04-24

### Fixed

- Fixed triggering reconciliation after adding finalizer to `ManagementClusterConfiguration` CRs.

## [0.3.0] - 2025-04-11

### Added

- Use a single `CiliumNetworkPolicy` to access Kubernetes API and allow traffic within the cluster.

### Removed

- Remove `NetworkPolicy` that only allowed access to the Flux `source-controller`. Replaced with above `CiliumNetworkPolicy`.

## [0.2.0] - 2025-04-11

### Added

- Support `.spec.reconciliation.suspend` on `ManagementClusterConfiguration` CRD.
- Support Helm chart value `.image.pullPolicy`, defaults to: `IfNotPresent`.

## [0.1.2] - 2025-03-28

### Fixed

- Fix map filtering logic used to respect external annotations and labels on generated resources.

## [0.1.1] - 2025-03-28

### Added

- Respect external labels on generated resources. An annotation or label is considered internal
  when it is prefixed with `configuration.giantswarm.io`. The state of internal annotations and labels is enforced
  on each reconciliation.

## [0.1.0] - 2025-03-12

### Added

- Initial implementation according to: https://github.com/giantswarm/rfc/pull/108

[Unreleased]: https://github.com/giantswarm/konfigure-operator/compare/v0.6.0...HEAD
[0.6.0]: https://github.com/giantswarm/konfigure-operator/compare/v0.5.1...v0.6.0
[0.5.1]: https://github.com/giantswarm/konfigure-operator/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/giantswarm/konfigure-operator/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/giantswarm/konfigure-operator/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/giantswarm/konfigure-operator/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/giantswarm/konfigure-operator/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/giantswarm/konfigure-operator/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/giantswarm/konfigure-operator/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/giantswarm/konfigure-operator/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/giantswarm/konfigure-operator/compare/v0.1.0...v0.1.0
