package main

import (
	"log"
	"os"

	"api/internal/database"
	"api/internal/handlers"
	"api/internal/middleware"
	"api/internal/models"
	"api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// Connect to database
	db, err := database.Connect()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Auto-migrate models
	if err := db.AutoMigrate(
		&models.User{},
		&models.Component{},
		&models.Incident{},
		&models.Subscriber{},
		&models.Translation{},
	); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Initialize email service
	emailService, err := services.NewEmailService()
	if err != nil {
		log.Printf("Warning: Failed to initialize email service: %v", err)
		log.Println("Notifications will not be sent. Please configure EMAIL_FROM and provider credentials.")
	}

	// Initialize notification service
	var notificationService *services.NotificationService
	if emailService != nil {
		notificationService = services.NewNotificationService(db, emailService)
		log.Println("Notification service initialized successfully")
	}

	// Create Gin router with default middleware (logger + recovery)
	router := gin.Default()

	// Public routes
	public := router.Group("/api/v1")
	{
		public.GET("/status", handlers.GetStatus(db))
		public.GET("/components", handlers.GetComponents(db))
		public.GET("/incidents", handlers.GetIncidents(db))

		// Subscription endpoints
		public.POST("/subscribe", handlers.Subscribe(db, emailService))
		public.GET("/confirm", handlers.Confirm(db))
		public.GET("/unsubscribe", handlers.Unsubscribe(db))

		// Authentication endpoint (public - no JWT required)
		public.POST("/admin/login", handlers.Login(db))
	}

	// Admin routes (protected with JWT middleware)
	admin := router.Group("/api/v1/admin")
	admin.Use(middleware.AuthRequired())
	{
		// Component management
		admin.GET("/components", handlers.ListComponents(db))
		admin.POST("/components", handlers.CreateComponent(db))
		admin.PUT("/components/:id", handlers.UpdateComponent(db))
		admin.DELETE("/components/:id", handlers.DeleteComponent(db))

		// Incident management
		admin.GET("/incidents", handlers.ListIncidents(db))
		admin.POST("/incidents", handlers.CreateIncident(db, notificationService))
		admin.PUT("/incidents/:id", handlers.UpdateIncident(db, notificationService))
		admin.DELETE("/incidents/:id", handlers.DeleteIncident(db))

		// Subscriber management
		admin.GET("/subscribers", handlers.ListSubscribers(db))
		admin.DELETE("/subscribers/:id", handlers.DeleteSubscriber(db))
	}

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Start server
	log.Printf("Starting server on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
