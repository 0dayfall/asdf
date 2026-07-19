package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/0dayfall/webfinger-server/internal/auth"
	"github.com/0dayfall/webfinger-server/internal/cache"
	"github.com/0dayfall/webfinger-server/internal/config"
	"github.com/0dayfall/webfinger-server/internal/middleware"
	"github.com/0dayfall/webfinger-server/internal/migrations"
	"github.com/0dayfall/webfinger-server/internal/monitoring"
	"github.com/0dayfall/webfinger-server/internal/rest"
	"github.com/0dayfall/webfinger-server/internal/store"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

const wellKnownWebFinger = "/.well-known/webfinger"

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

// Start initializes and starts the server.
//
// When no database URL is configured the server runs in standalone mode:
// WebFinger records are served from the JSON file at store.data_file with
// no PostgreSQL, Redis, or authentication required. Configuring
// WEBFINGER_DATABASE_URL enables the full stack.
func Start() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	metrics := monitoring.NewMetrics()
	if err := metrics.SetLogLevel(cfg.Logging.Level); err != nil {
		return fmt.Errorf("failed to set log level: %w", err)
	}
	logger := metrics.Logger()

	if cfg.Database.URL == "" {
		logger.Infof("Starting WebFinger Server (standalone mode, data file: %s)", cfg.Store.DataFile)
		return startStandalone(cfg, metrics, logger)
	}

	logger.Info("Starting WebFinger Server (database mode)")
	return startFull(cfg, metrics, logger)
}

// startStandalone serves WebFinger lookups from a JSON file: a single
// binary and a data file, nothing else to deploy.
func startStandalone(cfg *config.Config, metrics *monitoring.Metrics, logger *logrus.Logger) error {
	fileStore, err := store.NewFileStore(cfg.Store.DataFile)
	if err != nil {
		return err
	}
	logger.Infof("Loaded %d WebFinger record(s)", fileStore.Len())

	rest.LoadTemplates()
	htmlHandler := &rest.HTMLHandler{Data: fileStore}
	webfinger := &rest.WebFingerHandler{Data: fileStore}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", metrics.HealthCheckHandler())
	mux.Handle("/metrics", metrics.Handler())
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	mux.Handle(wellKnownWebFinger, webfinger)
	mux.Handle("/", htmlHandler)

	handler := applyMiddleware(mux, cfg, metrics, logger)
	addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
	return runServer(handler, addr, cfg, logger)
}

// startFull wires the PostgreSQL-backed server with optional Redis cache,
// authentication, and user management.
func startFull(cfg *config.Config, metrics *monitoring.Metrics, logger *logrus.Logger) error {
	// Connect to database
	pool, err := pgxpool.New(context.Background(), cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	// Run migrations
	migrator, err := migrations.NewMigrator(cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}
	defer migrator.Close()

	if !cfg.IsProductionEnv() {
		logger.Info("Running database migrations")
		if err := migrator.Up(); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}
	}

	// Initialize cache
	redisCache, err := cache.NewRedisCache(cfg.Redis.URL, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		logger.Warnf("Failed to connect to Redis: %v. Continuing without cache.", err)
		redisCache = nil
	}
	if redisCache != nil {
		defer redisCache.Close()
		logger.Info("Redis cache connected")
	}

	// Initialize stores
	webfingerStore := store.NewPostgresStore(pool)
	userStore := auth.NewPostgresUserStore(pool)
	sessionStore := auth.NewPostgresSessionStore(pool)

	// Initialize authentication service
	tokenExpiry := time.Duration(cfg.Auth.TokenExpiryHours) * time.Hour
	authService := auth.NewAuthService(cfg.Auth.JWTSecret, tokenExpiry, sessionStore)

	// Initialize handlers
	rest.LoadTemplates()
	htmlHandler := &rest.HTMLHandler{Data: webfingerStore}
	authHandler := rest.NewAuthHandler(authService, userStore)

	webfinger := &rest.WebFingerHandler{Data: webfingerStore}
	if redisCache != nil {
		// Only assign when non-nil: a typed nil in the interface would
		// bypass the handler's nil check.
		webfinger.Cache = redisCache
	}

	// Create main router
	mux := http.NewServeMux()

	// Health check (no auth required)
	mux.HandleFunc("/health", metrics.HealthCheckHandler())

	// Metrics endpoint (no auth required in development)
	mux.Handle("/metrics", metrics.Handler())

	// Static assets
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// Authentication endpoints
	mux.HandleFunc("/api/auth/login", authHandler.HandleLogin)
	mux.HandleFunc("/api/auth/register", authHandler.HandleRegister)
	mux.HandleFunc("/api/auth/logout", authHandler.HandleLogout)
	mux.HandleFunc("/api/auth/refresh", authHandler.HandleRefreshToken)

	// Protected API endpoints
	protectedMux := http.NewServeMux()
	protectedMux.HandleFunc("/api/profile", authHandler.HandleProfile)
	protectedMux.HandleFunc("/api/search", htmlHandler.HandleSearchAPI)

	// Admin endpoints (TODO: Add admin endpoints when needed)

	// WebFinger endpoint (public)
	mux.Handle(wellKnownWebFinger, webfinger)

	// HTML frontend
	mux.Handle("/", htmlHandler)

	handler := applyMiddleware(mux, cfg, metrics, logger)

	// Start background tasks
	ctx := context.Background()
	metrics.StartBackgroundTasks(ctx)

	// Start server
	addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)

	// Seed test data in development/test mode
	if cfg.IsTestEnv() || cfg.IsDevelopmentEnv() {
		if err := seedTestData(ctx, webfingerStore); err != nil {
			logger.Warnf("Failed to seed test data: %v", err)
		}
	}

	return runServer(handler, addr, cfg, logger)
}

// applyMiddleware wraps the router with the shared middleware chain
// (applied in reverse order).
func applyMiddleware(mux *http.ServeMux, cfg *config.Config, metrics *monitoring.Metrics, logger *logrus.Logger) http.Handler {
	rateLimiter := middleware.NewRateLimiter(cfg.Security.RateLimitRPS, cfg.Security.RateLimitBurst)

	var handler http.Handler = mux
	handler = middleware.Timeout(30 * time.Second)(handler)
	handler = middleware.Logging(logger.Infof)(handler)
	handler = metrics.HTTPMetricsMiddleware(handler)
	handler = middleware.SecurityHeaders(cfg.Security.EnableCSP, cfg.Security.EnableHSTS)(handler)
	handler = middleware.CORS(cfg.Security.AllowedOrigins, true)(handler)
	handler = middleware.TrustedProxies(cfg.Security.TrustedProxies)(handler)
	handler = rateLimiter.RateLimit(handler)

	if cfg.Security.ForceHTTPS && cfg.IsProductionEnv() {
		handler = middleware.HTTPSRedirect(true)(handler)
	}
	return handler
}

func runServer(handler http.Handler, addr string, cfg *config.Config, logger interface{ Infof(string, ...interface{}) }) error {
	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if cfg.IsTestEnv() || cfg.IsDevelopmentEnv() {
		logger.Infof("Starting HTTP server on %s (development mode)", addr)
		return server.ListenAndServe()
	} else {
		logger.Infof("Starting HTTPS server on %s (production mode)", addr)
		return server.ListenAndServeTLS(cfg.Server.CertPath, cfg.Server.KeyPath)
	}
}

// seedTestData creates test data for development and testing
func seedTestData(ctx context.Context, store *store.PostgresStore) error {
	// Check if example user already exists
	_, err := store.LookupBySubject(ctx, "acct:example@example.com")
	if err == nil {
		// User already exists
		return nil
	}

	// Create example user using the new schema
	return nil
}
