# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v2.3.0] - 2021-10-19
### Added
- Added new feature to command isolation test on one and only one fixture at a time.

## [v2.2.0] - 2021-09-14
### Added
- Added tabitha client for remote configuration management.

## [v2.1.3] - 2021-08-26
### Changed
- Reduce number of retries for faulted fixtures to one (from three) to reduce bad-recipe fallout.
- Update to latest towerproto (0.9.2)

## [v2.1.2] - 2021-07-12
### Changed
- Update to latest towerproto with new form_requests for completing or faulting a recipe.

## [v2.1.1] - 2021-04-22
### Changed
- Convert step_ordering field to bytes to reduce transmit size to FXR.

## [v2.1.0] - 2021-04-22
### Changed
- Update to latest towerproto.

### Fixed
- Create child logger in /{fixture}/{msgtype} endpoints instead of recursively overwriting the existing ones
  so it doesn't keep adding new additional fields with every request to the endpoint.

### Added
- Add long-recipe support by using the Cell API dedup feature and step ordering fields. This feature validated
  on-the-line on 2021-04-22.

## [v2.0.1] - 2021-03-22
### Fixed
- POST cell statuses on faulted fixture.

## [v2.0.0] - 2021-02-23
### Added
- Add power capacity, availability, and power in use metrics to availability endpoint. This changed the structure
  of the availability endpoint so is a breaking change.

## [v1.4.2] - 2021-02-11
### Changed
- Update to latest towerproto

## [v1.4.1] - 2021-02-09
### Changed
- Gather operational snapshot from last ACTIVE message, if one exists, instead of first faulted. This is
  because the first faulted message could be corrupted or incomplete if the fault was because the CIBs or
  STIBs went offline.

## [v1.4.0] - 2021-02-03
### Changed
- Ignore all recipe information from C/D Controller, as this information originated from Conductor.
  Conductor has shown a bug that results in incorrect tray information being associated with a tray ID.
  Fallout is that TC will run the wrong recipe on the tray due to the wrong recipe coming from CND. Now
  TC reaches out directly to cell API to get all this information to bypass the bug.

  Update MIN version as this is a robustness feature.

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

