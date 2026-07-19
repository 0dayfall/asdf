package server

import (
	"github.com/0dayfall/webfinger-server/internal/auth"
	"github.com/0dayfall/webfinger-server/internal/cache"
	"github.com/0dayfall/webfinger-server/internal/config"
	"github.com/0dayfall/webfinger-server/internal/middleware"
	"github.com/0dayfall/webfinger-server/internal/migrations"
	"github.com/0dayfall/webfinger-server/internal/monitoring"
	"github.com/0dayfall/webfinger-server/internal/rest"
	"github.com/0dayfall/webfinger-server/internal/store"
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const wellKnownWebFinger = "/.well-known/webfinger"

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

// Start initializes and starts the server with all features
func Start() error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize monitoring
	metrics := monitoring.NewMetrics()
	if err := metrics.SetLogLevel(cfg.Logging.Level); err != nil {
		return fmt.Errorf("failed to set log level: %w", err)
	}

	logger := metrics.Logger()
	logger.Info("Starting WebFinger Server")

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
	linkStore := store.NewPostgresLinkStore(pool)
	connStore := store.NewPostgresConnectionStore(pool)
	oauthStore := auth.NewPostgresOAuthStore(pool)

	// Initialize authentication service
	tokenExpiry := time.Duration(cfg.Auth.TokenExpiryHours) * time.Hour
	authService := auth.NewAuthService(cfg.Auth.JWTSecret, tokenExpiry, sessionStore)

	// Initialize OAuth service
	oauthService := auth.NewOAuthService(
		cfg.OAuth.BaseURL,
		cfg.OAuth.FacebookClientID, cfg.OAuth.FacebookSecret,
		cfg.OAuth.InstagramClientID, cfg.OAuth.InstagramSecret,
		cfg.OAuth.XClientID, cfg.OAuth.XSecret,
	)

	// Initialize handlers
	rest.LoadTemplates()
	htmlHandler := &rest.HTMLHandler{
		Data:      webfingerStore,
		Cache:     redisCache,
		Links:     linkStore,
		Conns:     connStore,
		Users:     userStore,
		AuthSvc:   authService,
	}
	authHandler := rest.NewAuthHandler(authService, userStore, cfg.Server.Domain)
	linksHandler := rest.NewLinksHandler(linkStore)
	connsHandler := rest.NewConnectionsHandler(connStore, userStore)
	oauthHandler := rest.NewOAuthHandler(oauthService, oauthStore, userStore, authService, redisCache, cfg.Server.Domain)

	// Setup middleware
	rateLimiter := middleware.NewRateLimiter(cfg.Security.RateLimitRPS, cfg.Security.RateLimitBurst)

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

	// Protected API endpoints — wrapped with auth middleware
	mux.Handle("/api/profile", authHandler.AuthMiddleware(http.HandlerFunc(authHandler.HandleProfile)))
	mux.Handle("/api/search", authHandler.AuthMiddleware(http.HandlerFunc(htmlHandler.HandleSearchAPI)))

	// Link management
	mux.Handle("/api/profile/links", authHandler.AuthMiddleware(http.HandlerFunc(linksHandler.HandleLinks)))
	mux.Handle("/api/profile/links/{id}", authHandler.AuthMiddleware(http.HandlerFunc(linksHandler.HandleLinkByID)))

	// Connections / friends
	mux.Handle("/api/connections", authHandler.AuthMiddleware(http.HandlerFunc(connsHandler.HandleConnections)))
	mux.Handle("/api/connections/pending", authHandler.AuthMiddleware(http.HandlerFunc(connsHandler.HandlePending)))
	mux.Handle("/api/connections/{id}", authHandler.AuthMiddleware(http.HandlerFunc(connsHandler.HandleConnectionByID)))

	// OAuth flows (public — redirect + callback)
	mux.HandleFunc("/auth/{provider}", oauthHandler.HandleOAuthRedirect)
	mux.HandleFunc("/auth/{provider}/callback", oauthHandler.HandleOAuthCallback)
	mux.HandleFunc("/auth/bluesky", oauthHandler.HandleBlueskyLogin)

	// Admin endpoints (TODO: Add admin endpoints when needed)

	// WebFinger endpoint (public)
	mux.HandleFunc(wellKnownWebFinger, htmlHandler.HandleWebFinger)

	// HTML pages
	mux.HandleFunc("/login", htmlHandler.LoginPageHandler)
	mux.HandleFunc("/register", htmlHandler.RegisterPageHandler)
	mux.HandleFunc("/dashboard", htmlHandler.DashboardPageHandler)
	mux.HandleFunc("/connections", htmlHandler.ConnectionsPageHandler)
	mux.Handle("/", htmlHandler)

	// Apply middleware chain
	var handler http.Handler = mux

	// Add middleware layers (applied in reverse order)
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

	handler = http.MaxBytesHandler(handler, 1<<20) // 1 MB request body limit

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
	// This replaces the old InitSchemaAndSeed method
	// Check if example user already exists
	_, err := store.LookupBySubject(ctx, "acct:example@example.com")
	if err == nil {
		// User already exists
		return nil
	}

	// Create example user using the new schema
	// This would need to be implemented in the store
	return nil
}
