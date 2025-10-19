# ASDF WebFinger Server

[![Go](https://github.com/0dayfall/asdf/actions/workflows/go.yml/badge.svg)](https://github.com/0dayfall/asdf/actions/workflows/go.yml)
[![GoDoc](https://godoc.org/github.com/0dayfall/asdf?status.png)](https://godoc.org/github.com/0dayfall/asdf)
[![license](http://img.shields.io/badge/license-GNU3-red.svg?)](https://raw.githubusercontent.com/0dayfall/asdf/LICENSE)

## Description

A comprehensive WebFinger server implementation following [RFC7033](https://datatracker.ietf.org/doc/html/rfc7033), featuring user management, authentication, caching, monitoring, and administrative capabilities.

## Features

### Core WebFinger
- ✅ RFC7033-compliant WebFinger endpoint (`/.well-known/webfinger`)
- ✅ JSON Resource Descriptor (JRD) responses
- ✅ Subject aliases and properties support
- ✅ Rel-typed links for social media verification
- ✅ HTML frontend for manual lookups

### Authentication & User Management
- ✅ JWT-based authentication with session management
- ✅ User registration and login system
- ✅ Password hashing with bcrypt
- ✅ Role-based access control (admin/user)
- ✅ Email verification support
- ✅ Session management with Redis storage

### Performance & Scalability
- ✅ Redis caching for WebFinger lookups
- ✅ Rate limiting with configurable limits
- ✅ Database connection pooling
- ✅ Graceful shutdown handling
- ✅ Request/response compression

### Security
- ✅ CORS configuration
- ✅ Security headers (CSP, HSTS, X-Frame-Options)
- ✅ HTTPS redirection (production mode)
- ✅ Input validation and sanitization
- ✅ Trusted proxy configuration

### Monitoring & Operations
- ✅ Prometheus metrics collection
- ✅ Structured JSON logging
- ✅ Health check endpoints
- ✅ Database migration system
- ✅ Admin dashboard with user management
- ✅ System statistics and monitoring

### Developer Experience
- ✅ Environment-based configuration
- ✅ Docker Compose setup with all dependencies
- ✅ Comprehensive test suite
- ✅ API documentation (OpenAPI/Swagger ready)
- ✅ Development vs production modes

## Quick Start

### Using Docker Compose (Recommended)

1. **Clone and setup:**
   ```bash
   git clone https://github.com/0dayfall/asdf.git
   cd asdf
   cp .env.example .env
   ```

2. **Generate certificates:**
   ```bash
   openssl genrsa -out server.key 2048
   openssl req -new -x509 -sha256 -key server.key -out server.crt -days 365
   mkdir -p certs && mv server.* certs/
   ```

3. **Generate secrets:**
   ```bash
   # Generate JWT and session secrets
   echo "ASDF_AUTH_JWT_SECRET=$(openssl rand -base64 32)" >> .env
   echo "ASDF_AUTH_SESSION_SECRET=$(openssl rand -base64 32)" >> .env
   ```

4. **Start services:**
   ```bash
   docker-compose up --build
   ```

### Manual Installation

1. **Prerequisites:**
   - Go 1.23+
   - PostgreSQL 16+
   - Redis 7+ (optional, for caching)

2. **Build and run:**
   ```bash
   go mod download
   go build -o asdf ./cmd/asdf
   ./asdf
   ```

## Configuration

Configuration can be provided via:
- Environment variables (prefixed with `ASDF_`)
- YAML configuration file (`config.yaml`)
- Command line arguments

### Key Configuration Options

```yaml
server:
  port: "8080"
  host: "0.0.0.0"
  env: "development"  # development, production, test

database:
  url: "postgres://user:pass@localhost/webfinger"
  max_open_conns: 25

redis:
  url: "redis://localhost:6379"
  password: ""
  db: 0

auth:
  jwt_secret: "your-secret-key"
  session_secret: "your-session-secret"
  token_expiry_hours: 24

security:
  rate_limit_rps: 10
  rate_limit_burst: 20
  allowed_origins: ["*"]
  enable_csp: true
  enable_hsts: true
  force_https: true
```

See `.env.example` for all available options.

## API Endpoints

### WebFinger
- `GET /.well-known/webfinger?resource=acct:user@domain.com` - WebFinger lookup

### Authentication
- `POST /api/auth/login` - User login
- `POST /api/auth/register` - User registration  
- `POST /api/auth/logout` - User logout
- `POST /api/auth/refresh` - Token refresh
- `GET /api/profile` - Get current user profile

### Search
- `GET /api/search?q=query` - Search users

### Admin (Requires Admin Role)
- `GET /api/admin/stats` - System statistics
- `GET /api/admin/users` - List users
- `POST /api/admin/users` - Create user
- `GET /api/admin/users/{id}` - Get user
- `PUT /api/admin/users/{id}` - Update user  
- `DELETE /api/admin/users/{id}` - Delete user

### Monitoring
- `GET /health` - Health check
- `GET /metrics` - Prometheus metrics

## Development

### Running Tests
```bash
go test ./...
```

### Database Migrations
```bash
# Run migrations
go run cmd/migrate/main.go up

# Rollback last migration  
go run cmd/migrate/main.go down

# Create new migration
go run cmd/migrate/main.go create add_new_feature
```

### Environment Modes

- **Development** (`GO_ENV=development`): HTTP server, auto-migrations, seed data
- **Test** (`GO_ENV=test`): In-memory database, HTTP server, auto-cleanup
- **Production** (`GO_ENV=production`): HTTPS server, manual migrations, security headers

## Monitoring

The server exposes Prometheus metrics at `/metrics`:

- HTTP request metrics (duration, status codes, paths)
- WebFinger-specific metrics (requests, cache hits/misses)
- Database connection metrics
- Authentication metrics (login attempts, active sessions)

### Using with Grafana

1. **Start monitoring stack:**
   ```bash
   docker-compose up prometheus grafana
   ```

2. **Access Grafana:** http://localhost:3000 (admin/admin)

3. **Add Prometheus datasource:** http://prometheus:9090

## Security Considerations

- Change default JWT and session secrets in production
- Use strong TLS certificates in production
- Configure proper CORS origins (not `*`)  
- Set up trusted proxy headers correctly
- Enable security headers (CSP, HSTS)
- Use rate limiting appropriate for your traffic
- Regularly backup your database
- Monitor authentication attempts and failed logins

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality  
4. Ensure all tests pass
5. Submit a pull request

## License

GNU General Public License v3.0 - see [LICENSE](LICENSE) file for details.
