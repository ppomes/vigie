package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"api/internal/cachetapi"
	"api/internal/database"
	"api/internal/handlers"
	"api/internal/importer"
	"api/internal/middleware"
	"api/internal/models"
	"api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// "api import-cachet ..." imports a Cachet 2.4 installation and exits
	if len(os.Args) > 1 && os.Args[1] == "import-cachet" {
		os.Exit(importer.RunCLI(os.Args[2:], func() (*gorm.DB, error) {
			db, err := database.Connect()
			if err != nil {
				return nil, err
			}
			// Keep the report readable: lookups that find nothing are
			// expected during an import.
			db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
			return db, database.Migrate(db)
		}, os.Stdout))
	}

	// Connect to database
	db, err := database.Connect()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Create and update the schema
	if err := database.Migrate(db); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Initialize audit service
	auditService := services.NewAuditService(db)

	// Initialize email service (try database config first, then environment variables)
	emailService, err := loadEmailServiceFromDB(db)
	if err != nil {
		log.Printf("No email config in database: %v", err)
		// Fall back to environment variables
		emailService, err = services.NewEmailService()
		if err != nil {
			log.Printf("Warning: Failed to initialize email service: %v", err)
			log.Println("Notifications will not be sent. Please configure email in admin settings or set environment variables.")
		}
	}

	// Initialize notification service with background workers
	var notificationService *services.NotificationService
	if emailService != nil {
		notificationService = services.NewNotificationService(db, emailService)
		notificationService.StartWorker()
		log.Println("Notification service initialized with background workers")
	} else {
		// Create notification service without email for queue stats (but won't send)
		notificationService = services.NewNotificationService(db, nil)
		log.Println("Notification service initialized without email (queue monitoring only)")
	}

	// Initialize maintenance status worker (auto-updates maintenance statuses based on time)
	maintenanceWorker := services.NewMaintenanceWorker(db)
	maintenanceWorker.Start()

	// Create Gin router with default middleware (logger + recovery)
	router := gin.Default()

	// Apply CORS middleware for frontend/admin access
	router.Use(middleware.CORS())

	// Public routes
	public := router.Group("/api/v1")
	{
		public.GET("/status", handlers.GetStatus(db))
		public.GET("/components", handlers.GetComponents(db))
		public.GET("/component-groups", handlers.GetComponentGroups(db))
		public.GET("/incidents", handlers.GetIncidents(db))
		public.GET("/maintenances", handlers.GetMaintenances(db))

		// Subscription endpoints (rate limited)
		public.POST("/subscribe", middleware.SubscribeLimiter.Middleware(), handlers.Subscribe(db, emailService))
		public.GET("/confirm", handlers.Confirm(db))
		public.GET("/unsubscribe", handlers.Unsubscribe(db))
		public.GET("/subscriptions", handlers.GetSubscription(db))
		public.PUT("/subscriptions", handlers.UpdateSubscription(db))

		// Settings endpoints (public)
		public.GET("/settings/theme", handlers.GetTheme(db))
		public.GET("/settings/branding", handlers.GetBranding(db))
		public.GET("/settings/customization", handlers.GetCustomization(db))
		public.GET("/settings/language", handlers.GetLanguage(db))

		// Authentication endpoint (public - no JWT required, rate limited)
		public.POST("/admin/login", middleware.LoginLimiter.Middleware(), handlers.Login(db, auditService))

		// Password reset endpoints (public, rate limited)
		public.POST("/forgot-password", middleware.PasswordResetLimiter.Middleware(), handlers.ForgotPassword(db, emailService, auditService))
		public.POST("/reset-password", middleware.PasswordResetLimiter.Middleware(), handlers.ResetPassword(db, auditService))
	}

	// Admin routes (protected with JWT middleware)
	admin := router.Group("/api/v1/admin")
	admin.Use(middleware.AuthRequiredWithDB(db))
	{
		// Self-service endpoints (any authenticated user)
		admin.GET("/me", handlers.GetCurrentUserProfile(db))
		admin.PUT("/me/password", middleware.RequireSession(), handlers.ChangeOwnPassword(db, auditService))
		twoFactor := admin.Group("/me/two-factor", middleware.RequireSession())
		twoFactor.POST("/setup", handlers.SetupTwoFactor(db, auditService))
		twoFactor.POST("/enable", handlers.EnableTwoFactor(db, auditService))
		twoFactor.POST("/disable", handlers.DisableTwoFactor(db, auditService))
		twoFactor.POST("/recovery-codes", handlers.RegenerateRecoveryCodes(db, auditService))

		// Read-only routes (viewers and admins can access)
		admin.GET("/components", handlers.ListComponents(db))
		admin.GET("/component-groups", handlers.ListComponentGroups(db))
		admin.GET("/incidents", handlers.ListIncidents(db))
		admin.GET("/subscribers", handlers.ListSubscribers(db))
		admin.GET("/settings", handlers.ListSettings(db))
		admin.GET("/templates", handlers.ListTemplates(db))
		admin.GET("/templates/:id", handlers.GetTemplate(db))
		admin.GET("/notifications/stats", handlers.GetNotificationStats(notificationService))
		admin.GET("/notifications/recent", handlers.GetRecentNotifications(notificationService))
		admin.GET("/maintenances", handlers.ListMaintenances(db))
		admin.GET("/maintenance-templates", handlers.ListMaintenanceTemplates(db))
		admin.GET("/maintenance-templates/:id", handlers.GetMaintenanceTemplate(db))
		admin.GET("/email", handlers.GetEmailConfig(db))

		// Admin-only routes (require admin role)
		adminOnly := admin.Group("")
		adminOnly.Use(middleware.RequireAdmin())
		{
			// Component management
			adminOnly.POST("/components", handlers.CreateComponent(db))
			adminOnly.PUT("/components/:id", handlers.UpdateComponent(db))
			adminOnly.DELETE("/components/:id", handlers.DeleteComponent(db))

			// Component groups
			adminOnly.POST("/component-groups", handlers.CreateComponentGroup(db))
			adminOnly.PUT("/component-groups/:id", handlers.UpdateComponentGroup(db))
			adminOnly.DELETE("/component-groups/:id", handlers.DeleteComponentGroup(db))

			// Incident management
			adminOnly.POST("/incidents", handlers.CreateIncident(db, notificationService))
			adminOnly.PUT("/incidents/:id", handlers.UpdateIncident(db, notificationService))
			adminOnly.DELETE("/incidents/:id", handlers.DeleteIncident(db))
			adminOnly.POST("/incidents/:id/updates", handlers.CreateIncidentUpdate(db, notificationService))
			adminOnly.PUT("/incidents/:id/updates/:update_id", handlers.UpdateIncidentUpdate(db))
			adminOnly.DELETE("/incidents/:id/updates/:update_id", handlers.DeleteIncidentUpdate(db))

			// Subscriber management
			adminOnly.DELETE("/subscribers/:id", handlers.DeleteSubscriber(db))

			// Settings management
			adminOnly.PUT("/settings/:key", handlers.UpdateSetting(db))

			// Email configuration
			adminOnly.PUT("/email", handlers.UpdateEmailConfig(db))
			adminOnly.POST("/email/test", middleware.EmailTestLimiter.Middleware(), handlers.TestEmailConfig(db))

			// Branding configuration
			adminOnly.PUT("/settings/branding", handlers.UpdateBranding(db))

			// Custom CSS, header and footer of the public page
			adminOnly.PUT("/settings/customization", handlers.UpdateCustomization(db))

			// Default language of the public page
			adminOnly.PUT("/settings/language", handlers.UpdateLanguage(db))

			// Incident templates
			adminOnly.POST("/templates", handlers.CreateTemplate(db))
			adminOnly.PUT("/templates/:id", handlers.UpdateTemplate(db))
			adminOnly.DELETE("/templates/:id", handlers.DeleteTemplate(db))
			adminOnly.POST("/templates/:id/preview", handlers.PreviewTemplate(db))

			// Notification queue
			adminOnly.POST("/notifications/retry", handlers.RetryFailedNotifications(notificationService))

			// Maintenance management
			adminOnly.POST("/maintenances", handlers.CreateMaintenance(db, notificationService))
			adminOnly.PUT("/maintenances/:id", handlers.UpdateMaintenance(db, notificationService))
			adminOnly.DELETE("/maintenances/:id", handlers.DeleteMaintenance(db))

			// Maintenance templates
			adminOnly.POST("/maintenance-templates", handlers.CreateMaintenanceTemplate(db))
			adminOnly.PUT("/maintenance-templates/:id", handlers.UpdateMaintenanceTemplate(db))
			adminOnly.DELETE("/maintenance-templates/:id", handlers.DeleteMaintenanceTemplate(db))
			adminOnly.POST("/maintenance-templates/:id/preview", handlers.PreviewMaintenanceTemplate(db))

			// User management (admin only). Changes need an interactive
			// session, so a leaked API key cannot create accounts or reset
			// passwords.
			adminOnly.GET("/users", handlers.ListUsers(db, auditService))
			adminOnly.GET("/users/:id", handlers.GetUser(db, auditService))
			adminOnly.POST("/users", middleware.RequireSession(), handlers.CreateUser(db, auditService))
			adminOnly.PUT("/users/:id", middleware.RequireSession(), handlers.UpdateUser(db, auditService))
			adminOnly.DELETE("/users/:id", middleware.RequireSession(), handlers.DeleteUser(db, auditService))
			adminOnly.PUT("/users/:id/password", middleware.RequireSession(), handlers.ResetUserPassword(db, auditService))
			adminOnly.DELETE("/users/:id/two-factor", middleware.RequireSession(), handlers.ResetUserTwoFactor(db, auditService))

			// API keys (admin only, and only from an interactive session:
			// an API key cannot create or revoke keys)
			apiKeys := adminOnly.Group("/api-keys", middleware.RequireSession())
			apiKeys.GET("", handlers.ListAPIKeys(db))
			apiKeys.POST("", handlers.CreateAPIKey(db, auditService))
			apiKeys.DELETE("/:id", handlers.RevokeAPIKey(db, auditService))

			// Audit logs (admin only)
			adminOnly.GET("/audit", handlers.ListAuditLogs(auditService))
			adminOnly.GET("/audit/export", handlers.ExportAuditLogs(auditService))
		}
	}

	// Cachet 2.x compatible API, for tools written for Cachet
	cachetapi.Register(router, db, notificationService)

	// Cachet 2.4 public URLs (incident links, subscription links, feeds),
	// forwarded here by the public site
	handlers.RegisterCachetRedirects(router, db)

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting server on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Stop notification workers
	if notificationService != nil {
		notificationService.StopWorker()
	}

	// Stop maintenance worker
	maintenanceWorker.Stop()

	// Graceful shutdown with 10 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

// loadEmailServiceFromDB loads email configuration from database and creates an email service
func loadEmailServiceFromDB(db *gorm.DB) (services.EmailService, error) {
	var setting models.Settings
	if err := db.Where("`key` = ?", "email_config").First(&setting).Error; err != nil {
		return nil, err
	}

	var config services.EmailConfig
	if err := json.Unmarshal([]byte(setting.Value), &config); err != nil {
		return nil, err
	}

	if config.Provider == "" || config.Provider == "disabled" {
		return nil, nil
	}

	emailService, err := services.NewEmailServiceFromConfig(&config)
	if err != nil {
		return nil, err
	}

	log.Printf("Email service loaded from database (provider: %s)", config.Provider)
	return emailService, nil
}
