# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

