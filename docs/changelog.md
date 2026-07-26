# Changelog

## - 2026-06-26

### Added
- feature/auth-001
- add zap logger wrapper with leveled logging
- started project, created base infrastructure and documentation.

## - 2026-06-28

### Added
- feature/auth-002-me-handler-impl
- feat: implemented me handler flow via JWT token

Added - **auth**: Implemented Refresh Token storage in Redis with automatic TTL expiration.
- **middleware**: Added Redis-backed rate limiting for the `/me` endpoint handler.


## - 2026-07-12

### Added

- feature/auth-003-gitlab-ci
- added Kubernetes manifests (Deployment, Service) for auth-service, auth-db (Postgres) and redis
- set up GitLab CI pipeline: build, test, deploy stages
- deploy stage applies manifests to a k8s cluster for verification

## - 2026-07-13

### Added

- feature/auth-004-implemet-ingress-for-outer-requests
- added access from other outer networks

### Added

- feature/auth-005-add-migration-pipeline
- add DB migration pipeline and fixed db flow with registration

### Added

- feature/auth-006-add-prometheus-grafana-monitoring
- add Prometheus and Grafana deployments


### Added

- feature/auth-007-tests
- add tests for auth service generate Token
