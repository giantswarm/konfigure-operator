# Chaqngelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/giantswarm/konfigure-operator/compare/v0.1.2...HEAD
[0.1.2]: https://github.com/giantswarm/konfigure-operator/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/giantswarm/konfigure-operator/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/giantswarm/konfigure-operator/compare/v0.1.0...v0.1.0
