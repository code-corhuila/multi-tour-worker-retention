# Changelog

All notable changes of `multi-tour-worker-retention` (Data retention worker) are documented in this file.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [3.1.0] - 2026-11-18

Minor release after 3.0.0. The entry below is a pull request merged into `qa` and re-applied to `release/3.1.0` with `git cherry-pick -x`; it names no user story, so it is grouped under the story of this component, and the entry is the title of the pull request without its type. The frozen head of `qa` is `e136a75ae181383c684110dd41b077171478c743`.

### Changed

#### HU-WRK-006 - Apply data retention

- Add how to start and test to the readme and the pr template ([PR 12](https://github.com/code-corhuila/multi-tour-worker-retention/pull/12))

## [3.0.0] - 2026-11-14

Cut 3 release. Every entry below is a pull request merged into `qa` and re-applied to `release/3.0.0` with `git cherry-pick -x`; the heading of each group is the user story named in the pull request, and the entry is the title of the pull request without its type. The frozen head of `qa` is `b5a4a0fdc2ed016d68b5fa52f1488a9915bcd234`.

### Changed

#### HU-WRK-006 - Apply data retention

- The module and configuration of the retention worker (part 1) ([PR 4](https://github.com/code-corhuila/multi-tour-worker-retention/pull/4))
- The job and its adapters of the retention worker (part 2) ([PR 5](https://github.com/code-corhuila/multi-tour-worker-retention/pull/5))
- HU-WRK-006 ([PR 7](https://github.com/code-corhuila/multi-tour-worker-retention/pull/7))
- Pin the worker runtime at its first version v0.1.0 ([PR 9](https://github.com/code-corhuila/multi-tour-worker-retention/pull/9))

[Unreleased]: https://github.com/code-corhuila/multi-tour-worker-retention/compare/v3.1.0...HEAD
[3.1.0]: https://github.com/code-corhuila/multi-tour-worker-retention/releases/tag/v3.1.0
[3.0.0]: https://github.com/code-corhuila/multi-tour-worker-retention/releases/tag/v3.0.0
