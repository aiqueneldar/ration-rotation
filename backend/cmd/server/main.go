package main

import (
	"context"
	"log"
	"maps"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"sync"
	"syscall"
	"time"

	"ration-rotation/internal/api"
	"ration-rotation/internal/config"
	"ration-rotation/internal/db"
	"ration-rotation/internal/store"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/netresearch/go-cron"
	"golang.org/x/time/rate"
)

// HTTPServer wraps http.Server for graceful shutdown with Gin integration
type HTTPServer struct {
	server   *http.Server
	cancel   context.CancelFunc
	cron     *cron.Cron
	warnings *api.WarningsService
}

func (h *HTTPServer) Shutdown(ctx context.Context) error {
	h.cancel()

	// Stop cron jobs
	if h.cron != nil {
		h.cron.Stop()
	}

	return h.server.Shutdown(ctx)
}

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	time.LoadLocation(cfg.Server.TZ)

	// Initialize database
	if err := db.InitDB(cfg.Database.URL); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.CloseDB()

	// Create tables if needed
	if cfg.Database.CreateTablesIfMissing {
		if err := db.CreateTablesIfMissing(); err != nil {
			log.Fatalf("Failed to create database tables: %v", err)
		}
	} else {
		log.Println("Database tables will not be auto-created. Create manually or set CREATE_TABLES_IF_MISSING=true")
	}

	// Initialize stores and services
	itemStore := store.NewItemStore(db.DB)

	// Initialize SignalClient
	var signalServer *api.SignalServer
	if cfg.Signal.Endpoint != "" {
		signalServer = &api.SignalServer{
			Endpoint: cfg.Signal.Endpoint,
		}
	} else {
		signalServer = &api.SignalServer{}
	}

	warningsService := api.NewWarningsService(itemStore, signalServer, cfg.Warning.CheckIntervalMinutes, cfg)

	// Set up Gin router with middleware
	router := setupGinRouter(handlers{handlers: api.NewHandlers(itemStore), warnings: warningsService, signal: signalServer}, cfg)

	// Create HTTP server wrapper with Gin handler
	httpSrv := &HTTPServer{
		server: &http.Server{
			Addr:         cfg.Server.Address + ":" + strconv.Itoa(cfg.Server.Port),
			Handler:      router,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		warnings: warningsService,
	}

	// Start cron job for checking expiring items (every X minutes)
	if cfg.Warning.CheckIntervalMinutes > 0 {
		c := cron.New()
		intervalStr := strconv.Itoa(cfg.Warning.CheckIntervalMinutes)
		// Cron expects: minute hour day-of-month month day-of-week
		// Pattern: "minute hour day-of-month month day-of-week"
		cronPattern := "*/" + intervalStr + " * * * *" // */interval * * * * = Every interal minute every hour of every day of every month of every week
		_, err = c.AddFunc(cronPattern, func() {
			log.Println("Running warning check...")
			if err := httpSrv.warnings.CheckAndNotify(); err != nil {
				log.Printf("Warning check error: %v", err)
			}
		})
		if err != nil {
			log.Printf("Failed to add cron job: %v", err)
		} else {
			httpSrv.cron = c
			c.Start()
			log.Println("Cron job started for warning checks")
		}
	}

	// Start server in goroutine
	go func() {
		log.Printf("Server listening on %s", httpSrv.server.Addr)
		if err := httpSrv.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	<-shutdown

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	httpSrv.cancel = cancel
	defer cancel()

	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
		return
	}

	log.Println("Server shut down gracefully")
}

// handlers wraps both handlers and warnings service for Gin route setup
type handlers struct {
	handlers *api.Handlers
	warnings *api.WarningsService
	signal   *api.SignalServer
}

// setupGinRouter sets up the Gin router with all API endpoints following Gin best practices
func setupGinRouter(hdlr handlers, cfg *config.Config) *gin.Engine {
	// Set Gin mode to release for production (removes debug logging and reduces memory footprint)
	gin.SetMode(gin.ReleaseMode)

	// Create engine
	router := gin.New()

	// API routes - Group all /api/* routes (allows applying middleware to the whole group if needed)
	api := router.Group("/api")

	//Add security middle ware and fixs
	api.Use(cors.New(cors.Config{
		AllowOrigins:     slices.Collect(maps.Keys(cfg.Server.CORSOrigins)),
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowHeaders:     []string{"Origin", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "Forwarded", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           1 * time.Hour,
	}))
	api.Use(RateLimiter())

	// Items endpoints
	items := api.Group("/items")

	items.GET("", hdlr.handlers.GetItems)
	items.POST("", hdlr.handlers.CreateItem)
	items.GET("/barcode/:barcode", hdlr.handlers.GetItemByBarcode)
	items.GET("/:id", hdlr.handlers.GetByID)
	items.PATCH("/:id", hdlr.handlers.UpdateItem)
	items.DELETE("/:id", hdlr.handlers.DeleteItem)
	items.POST("/:id/image", hdlr.handlers.UploadImage)
	items.PATCH("/:id/snooze", hdlr.handlers.SnoozeItem)
	items.PATCH("/:id/rotate", hdlr.handlers.RotateItem)

	// Warnings
	api.GET("/warnings", hdlr.warnings.GetExpiringItems)
	api.GET("/warnings/check", func(c *gin.Context) {
		if err := hdlr.warnings.TriggerCheck(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"msg": "Warning check failed", "error": err})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Warning check triggered"})
	})

	// Link signal
	api.POST("/link", hdlr.signal.LinkingHandlerStart)
	api.GET("/link/status", hdlr.signal.LinkingStatus)

	// Health check (not behind /api group - for nginx load balancers to verify backend is alive)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	return router
}

func RateLimiter() gin.HandlerFunc {
	type client struct {
		limiter *rate.Limiter
	}

	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)

	return func(c *gin.Context) {
		ip := c.ClientIP()

		mu.Lock()
		if _, exists := clients[ip]; !exists {
			// Allow 10 requests per second with a burst of 20
			clients[ip] = &client{limiter: rate.NewLimiter(10, 20)}
		}
		cl := clients[ip]
		mu.Unlock()

		if !cl.limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			return
		}

		c.Next()
	}
}
