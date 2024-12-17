package main

import (
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cache"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/template/html/v2"
)

type PageData struct {
	Series          []SimplifiedSeries
	UpcomingSeries  []SimplifiedUpcomingSeries
}

type WebAPIResponse struct {
	Data  interface{} `json:"data"`
	Error string      `json:"error,omitempty"`
}

func main() {
	// Set up logging to file
	logFile, err := os.OpenFile("server.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal("Failed to open log file:", err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	
	// Initialize template engine
	engine := html.New("./views", ".html")
	
	// Create Fiber app with more verbose error logging
	app := fiber.New(fiber.Config{
		Views: engine,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			log.Printf("Error handling request: %v", err)
			return c.Status(500).JSON(WebAPIResponse{
				Error: err.Error(),
			})
		},
	})

	// Add logging middleware with more detailed format
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${method} ${path} - ${ip} - ${latency}\n",
	}))

	// Add CORS middleware with more permissive settings
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,HEAD,PUT,DELETE,PATCH",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))

	// Add cache middleware for API routes
	apiCache := cache.New(cache.Config{
		Next: func(c *fiber.Ctx) bool {
			return c.Path() == "/" // Skip caching for the main page
		},
		Expiration: 15 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.Path() // Use the path as the cache key
		},
	})

	// Serve static files (including cached images)
	app.Static("/static", "./static")

	// API endpoints for data with caching
	app.Get("/api/series", apiCache, func(c *fiber.Ctx) error {
		log.Printf("Received request for /api/series from %s", c.IP())
		series, err := GetSeriesData()
		if err != nil {
			log.Printf("Error getting series data: %v", err)
			return c.Status(500).JSON(WebAPIResponse{
				Error: err.Error(),
			})
		}
		
		// Transform image URLs to use cached versions
		transformed, err := transformImageURLs(
			series,
			func(s SimplifiedSeries) string { return s.Image },
			func(s SimplifiedSeries, url string) SimplifiedSeries {
				s.Image = url
				return s
			},
		)
		if err != nil {
			log.Printf("Error transforming image URLs: %v", err)
			// Continue with original series data if transformation fails
			transformed = series
		}
		
		log.Printf("Successfully fetched and transformed %d series", len(transformed))
		
		// Log the first few series and their cached image URLs for debugging
		for i, s := range transformed {
			if i < 3 { // Log only first 3 for brevity
				log.Printf("Series %d: %s, Cached Image URL: %s", i+1, s.Name, s.Image)
			}
		}
		
		return c.JSON(WebAPIResponse{
			Data: transformed,
		})
	})

	app.Get("/api/upcoming-series", apiCache, func(c *fiber.Ctx) error {
		log.Printf("Received request for /api/upcoming-series from %s", c.IP())
		upcomingSeries, err := GetUpcomingSeriesData()
		if err != nil {
			log.Printf("Error getting upcoming series data: %v", err)
			return c.Status(500).JSON(WebAPIResponse{
				Error: err.Error(),
			})
		}
		
		// Transform image URLs to use cached versions
		transformed, err := transformImageURLs(
			upcomingSeries,
			func(s SimplifiedUpcomingSeries) string { return s.Image },
			func(s SimplifiedUpcomingSeries, url string) SimplifiedUpcomingSeries {
				s.Image = url
				return s
			},
		)
		if err != nil {
			log.Printf("Error transforming image URLs: %v", err)
			// Continue with original series data if transformation fails
			transformed = upcomingSeries
		}
		
		log.Printf("Successfully fetched and transformed %d upcoming series", len(transformed))
		
		// Log the first few series and their cached image URLs for debugging
		for i, s := range transformed {
			if i < 3 { // Log only first 3 for brevity
				log.Printf("Upcoming Series %d: %s, Cached Image URL: %s", i+1, s.Name, s.Image)
			}
		}
		
		return c.JSON(WebAPIResponse{
			Data: transformed,
		})
	})

	// Main route now just serves the template
	app.Get("/", func(c *fiber.Ctx) error {
		log.Printf("Serving index page to %s", c.IP())
		return c.Render("index", fiber.Map{})
	})

	log.Printf("Server starting on http://localhost:3001")
	// Start server on port 3001
	log.Fatal(app.Listen(":3001"))
}
