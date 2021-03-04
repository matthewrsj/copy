# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v1.2.1] - 2021-03-03
### Changed
- Convert to new API for start/end step.

## [v1.2.0] - 2021-02-23
### Added
- No longer route to towers that do not have enough power capacity for the tray(s) being routed.

## [v1.1.4] - 2021-02-11
### Fixed
- Do not place tray in column 16 with the back fork, as this is not physically possible.

## [v1.1.3] - 2021-01-26
### Changed
- All non-production trays now use round-robin routing algorithm since non-production trays
  are used to stress equipment. RR routing utilizes crane and conveyor equipment equally.

### Removed
- Do not use Conductor recipe to send to tower controller, let tower controller query for a recipe
  itself. This bypasses a bug in Conductor that has been difficult to track down.

## [v1.1.2]
### Added
- Ability to generate cell map without considering status codes for commission recipe
  purposes.

## [v1.1.1]
### Fixes
- Remove guard to placing in last column, as this caused a nil-pointer dereference.
- Better logging in tower selection.

### Added
- OPTIONS to CORS allowances
- HoldTray and ReleaseTray helpers for egressing trays that are faulting everything

## [v1.1.0]
### Added
- Fault management system to track Operational snapshots for trays that ended on a fixture fault.
  This allows the tray to be resumed in a new fixture where it left off in the old one.
- Add POST and other methods to CORS allowance.
- Add ability to POST cell statuses without closing the step.

### Changed
- Retry load command for five minutes before rejecting load to CND.
- Route to aisles based on aisle availability instead of round robin, but leave round robin routing
  in place behind a --demo flag.

## [v1.0.3]
### Added
- Enable CORS on API endpoints so UI can query

### Removed
- No longer alarm to CND on fire alarm, this is handled by watchtower now

### Changed
- Reject to CND with State_Type Current instead of Desired

## [v1.0.2] - 2020-12-03
### Fixed
- Add 10 second timeout when requesting tower availability, so this operation can't hang forever.

## [v1.0.1] - 2020-12-02
### Fixed
- Lock reload requests so they are serialized and don't race to the same fixture

## [v1.0.0] - 2020-11-24
### Added
- Added CHANGELOG.md

