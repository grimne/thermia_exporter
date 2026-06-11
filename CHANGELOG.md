# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.1] - 2026-06-11

### Added

- Access tokens are now renewed via the OAuth2 refresh-token grant — a single
  lightweight token call roughly once an hour. The full password login flow is
  only used at startup, or as an automatic fallback if the refresh is rejected.
- GitHub Releases are created automatically on tag push, with release notes
  taken from this changelog.

### Fixed

- The container image no longer includes provenance attestation manifests,
  which displayed as an extra `unknown/unknown` OS/Arch entry on GHCR. The
  published image is `linux/amd64` only.

## [0.2.0] - 2026-06-11

### Changed

- **Collection is decoupled from scraping.** The exporter now polls the
  Thermia API in a background loop and serves `/metrics` from an in-memory
  cache. Scrapes complete in milliseconds and can no longer time out on slow
  upstream responses, which previously caused intermittent `up == 0` false
  alerts.
- The `THERMIA_SCRAPE_INTERVAL` environment variable (seconds, default `900`,
  minimum `60`) now controls the background collection interval. It was
  previously documented but unused.
- Upstream API call volume is reduced accordingly: one collection per
  interval, independent of how often Prometheus scrapes.

### Added

- New gauge `thermia_last_collection_success_timestamp_seconds` for staleness
  monitoring and alerting.

### Fixed

- A failed collection now keeps serving the previous result and increments
  `thermia_scrape_errors_total`, instead of silently returning an empty
  scrape.

## [0.1.2] - 2025-12-30

Initial public container release.
