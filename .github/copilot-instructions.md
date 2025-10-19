# Copilot Instructions for asdf

## Overview

This project implements a RFC7033-compliant WebFinger server in Go, providing user discovery via `acct:` URIs with both API endpoints and an HTML frontend for manual lookups.

## Architecture

- **Entry Point:** `cmd/asdf/main.go` requires `PORT`, `SSL_CERT_PATH`, and `SSL_KEY_PATH` environment variables
- **Server:** `internal/server/server.go` configures routes, TLS, and environment-based behavior (`GO_ENV=test` enables HTTP mode and auto-seeding)
- **WebFinger Handler:** `internal/rest/html_handler.go` serves both `/.well-known/webfinger` API and HTML frontend via unified handler
- **Resource Parsing:** `internal/resource/` validates `acct:` resources, requiring `@` symbol for validity
- **Data Layer:** `internal/store/` provides `Store` interface with Postgres implementation, auto-creates schema in test mode
- **Types:** `internal/types/jrd.go` defines JRD (JSON Resource Descriptor) with Subject, Aliases, Properties, Links
- **Frontend:** `web/template/` contains search and account display templates, `web/static/` serves CSS

## Key Workflows

- **Local Development:**
  - `docker-compose up --build` starts both web server and Postgres DB
  - Requires `.env` file with `PORT`, `DATABASE_URL`, `SSL_CERT_PATH`, `SSL_KEY_PATH`
  - Generate TLS certs: `openssl genrsa -out server.key 2048 && openssl req -new -x509 -sha256 -key server.key -out server.crt -days 365`
- **Testing:**
  - Set `GO_ENV=test` for HTTP mode (no TLS) and automatic schema initialization
  - Uses `github.com/stretchr/testify/require` for assertions
  - Run with `go test ./...` - tests use mock store for isolation
- **WebFinger Queries:**
  - API: `GET /.well-known/webfinger?resource=acct:user@domain.com`
  - Returns JRD with subject, aliases, properties, and rel-typed links
  - HTML frontend at `/` for manual user lookups

## Patterns & Conventions

- **Unified Handler:** `HTMLHandler` serves both WebFinger API and HTML UI from same struct via method routing
- **Resource Validation:** `resource.ParseResource()` strips `acct:` prefix, validates `@` presence for email-like identifiers
- **Environment-Based Modes:** `GO_ENV=test` switches HTTP/HTTPS, enables auto-seeding of `acct:example@example.com`
- **Store Interface:** Dependency injection with `LookupBySubject()` and `SearchSubjects()` methods for testability
- **Template Loading:** `LoadTemplates()` called at startup, templates executed directly in handlers
- **JSON Responses:** WebFinger returns `application/jrd+json`, search API returns `application/json`

## Integration Points

- **Database:** Postgres via `github.com/jackc/pgx/v5/pgxpool` with JSONB columns for properties/links
- **WebFinger Protocol:** Strict RFC7033 compliance - `/.well-known/webfinger` endpoint with `resource` query param
- **Docker Stack:** `docker-compose.yml` orchestrates web server + Postgres with volume persistence
- **Static Assets:** `/static/` route serves `web/static/` directory (CSS, JS, images)

## Critical Details

- **Subject Format:** Resources must be `acct:` URIs that resolve to email-like identifiers with `@`
- **Database Schema:** Auto-created `users` table with `subject`, `aliases[]`, `properties` (JSONB), `links` (JSONB)
- **Test Mode:** `GO_ENV=test` bypasses TLS, auto-seeds example user, enables HTTP-only operation
- **Mock Testing:** `store.NewMockStore()` provides in-memory implementation for unit tests
