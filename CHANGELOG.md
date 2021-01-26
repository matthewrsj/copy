# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v1.3.1] - 2021-01-26
### Changed
- Update to latest towerproto

## [v1.3.0] - 2021-01-26
### Changed
- Now query cell API directly for recipe using cdcontroller library. No longer rely on message
  coming from Conductor via C/D Controller. Improves robustness.

### Fixed
- Only accept one preparedForDelivery or loadRequest per fixture at a time. Do not queue
  them up, as this will only result in double-handling of a tray that is no longer there.

## [v1.2.2] - 2021-01-12
### Changed
- Increase data expiry to 7 seconds for new variable data rate.

## [v1.2.1] - 2021-01-12
### Added
- Add orientation to proto message for fault record.

### Fixed
- Retry retrieving the cell map if network goes down.

## [v1.2.0] - 2020-12-29
### Added
- New feature: egress trays to manual inspection station when fixture fault limit reached
- Add OPTIONS to CORS methods

### Fixed
- Do not exclude cells from cell map for commissioning trays. This way all locations will
  continue to be tested.

## [v1.1.1] - 2020-12-22
### Fixed
- Internally record fixture fault so cell status post does not close step

## [v1.1.0] - 2020-12-22
### Added
- Allow CORS requests
- Add recipe fault recovery when a tray has faulted on another fixture.

### Changed
- POST cell statuses to Cell API even when there's a fault, but do not close step.

### Removed
- No longer request unload when a fixture faults due to fire alarm, as this is handled by
  watchtower.
- [configuration] Turn off log duplication by docker to prevent disk running out of space.

## [v1.0.3] - 2020-12-08
### Removed
- No longer command fire suppression response from TC, it is now handled by watchtower service.

## [v1.0.2] - 2020-12-03
### Changed
- Update to protostream v1.0.1
- Update to towerproto v0.6.0

### Fixed
- Prevent double-load requests by locking the endpoint with a mutex.
- Update network retries to retry forever instead of for the default 15 minutes.

## [v1.0.1] - 2020-11-30
### Fixed
- No longer double-close step by using /next endpoint. Only post cell statuses.
- Upon incorrect load message from C/D Controller unload the tray instead of
  forgetting about it.
- Fix logger bug for preparedForDelivery endpoint causing increasingly longer
  log messages.
## [v1.0.0] - 2020-11-24
### Added
- Added CHANGELOG.md

