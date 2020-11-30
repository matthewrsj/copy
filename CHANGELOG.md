# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

