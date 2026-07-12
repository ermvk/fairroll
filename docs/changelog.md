# Changelog

## - 2026-06-26

### Added
- feature/auth-001
- add zap logger wrapper with leveled logging
- started project, created base infrastructure and documentation.

## - 2026-06-28

### Added
- feature/auth-002-me-handler-impl
  feat: implemented me handler flow via JWT token

Added - **auth**: Implemented Refresh Token storage in Redis with automatic TTL expiration.
- **middleware**: Added Redis-backed rate limiting for the `/me` endpoint handler.


## - 2026-07-12

### Added

- feature/auth-003-gitlab-ci
- added Kubernetes manifests (Deployment, Service) for auth-service, auth-db (Postgres) and redis
- set up GitLab CI pipeline: build, test, deploy stages
- deploy stage applies manifests to a k8s cluster for verification