package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cache"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/template/html/v2"
)

type PageData struct {
	Series          []SimplifiedSeries
	UpcomingSeries  []SimplifiedSeries
}

type WebAPIResponse struct {
	Data  interface{} `json:"data"`
	Error string      `json:"error,omitempty"`
}

func main() {
	// Ensure cache directory exists
	if err := os.MkdirAll("static/cache", 0755); err != nil {
		log.Fatal("Failed to create cache directory:", err)
	}

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
		allSeries, err := GetSeriesData()
		if err != nil {
			log.Printf("Error getting series data: %v", err)
			return c.Status(500).JSON(WebAPIResponse{
				Error: err.Error(),
			})
		}

		// Filter for series with firstAired <= today
		var currentSeries []SimplifiedSeries
		today := time.Now()
		for _, series := range allSeries {
			firstAired, err := time.Parse("2006-01-02", series.FirstAired)
			if err != nil {
				continue
			}
			if !firstAired.After(today) {
				currentSeries = append(currentSeries, series)
			}
		}
		
		// Transform image URLs to use cached versions
		transformed, err := transformImageURLs(
			currentSeries,
			func(s SimplifiedSeries) string { return s.Image },
			func(s SimplifiedSeries) int { return s.ID },
			func(s SimplifiedSeries, url string) SimplifiedSeries {
				s.Image = url
				return s
			},
		)
		if err != nil {
			log.Printf("Error transforming image URLs: %v", err)
			// Continue with original series data if transformation fails
			transformed = currentSeries
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
		allSeries, err := GetSeriesData()
		if err != nil {
			log.Printf("Error getting upcoming series data: %v", err)
			return c.Status(500).JSON(WebAPIResponse{
				Error: err.Error(),
			})
		}

		// Filter for series with firstAired > today and valid image URL
		var upcomingSeries []SimplifiedSeries
		today := time.Now()
		for _, series := range allSeries {
			firstAired, err := time.Parse("2006-01-02", series.FirstAired)
			if err != nil {
				continue
			}
			// Check if series is upcoming and has a valid image URL
			hasValidImage := series.Image != "" && strings.HasPrefix(series.Image, "https://")
			if firstAired.After(today) && hasValidImage {
				upcomingSeries = append(upcomingSeries, series)
			}
		}
		
		// Transform image URLs to use cached versions
		transformed, err := transformImageURLs(
			upcomingSeries,
			func(s SimplifiedSeries) string { return s.Image },
			func(s SimplifiedSeries) int { return s.ID },
			func(s SimplifiedSeries, url string) SimplifiedSeries {
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

	// New endpoint for getting detailed series information
	app.Get("/api/series/:id", apiCache, func(c *fiber.Ctx) error {
		seriesID, err := strconv.Atoi(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(WebAPIResponse{
				Error: "Invalid series ID",
			})
		}

		log.Printf("Received request for series details ID: %d from %s", seriesID, c.IP())
		seriesInfo, err := GetSeriesInfo(seriesID)
		if err != nil {
			log.Printf("Error getting series info: %v", err)
			return c.Status(500).JSON(WebAPIResponse{
				Error: err.Error(),
			})
		}

		// Transform image URL to use cached version
		if seriesInfo.Image != "" {
			transformed, err := transformImageURLs(
				[]DetailedSeries{*seriesInfo},
				func(s DetailedSeries) string { return s.Image },
				func(s DetailedSeries) int { return s.ID },
				func(s DetailedSeries, url string) DetailedSeries {
					s.Image = url
					return s
				},
			)
			if err != nil {
				log.Printf("Error transforming image URL: %v", err)
			} else if len(transformed) > 0 {
				seriesInfo = &transformed[0]
			}
		}

		// Fetch and cache the background image
		backgroundURL, err := GetSeriesBackground(seriesID)
		if err != nil {
			log.Printf("Error getting background image: %v", err)
		} else {
			// Cache the background image
			err = cacheImage(backgroundURL, fmt.Sprintf("%d-background.jpg", seriesID))
			if err != nil {
				log.Printf("Error caching background image: %v", err)
			}
		}

		return c.JSON(WebAPIResponse{
			Data: seriesInfo,
		})
	})

	// Main route now just serves the template
	app.Get("/", func(c *fiber.Ctx) error {
		log.Printf("Serving index page to %s", c.IP())
		return c.Render("index", fiber.Map{})
	})

	// New route to serve the series detail page
	app.Get("/series/:id", func(c *fiber.Ctx) error {
		log.Printf("Serving series detail page for ID %s to %s", c.Params("id"), c.IP())
		return c.Render("series-detail", fiber.Map{})
	})

	log.Printf("Server starting on http://localhost:3002")
	// Start server on port 3002
	log.Fatal(app.Listen(":3002"))
}
