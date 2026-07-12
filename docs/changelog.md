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

- git checkout -b feature/auth-003-gitlab-ci