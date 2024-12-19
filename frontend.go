package main

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"log"
	"sync"
	"time"
)

var (
	cacheMutex       sync.Mutex
	lastCacheTime    time.Time
	cachedSeriesIds  = make(map[int]bool)
	cacheInProgress  bool
)

// cacheSeriesContent handles caching all content for a single series
func cacheSeriesContent(series SimplifiedSeries, wg *sync.WaitGroup) {
	defer wg.Done()

	// Check if this series has already been cached
	cacheMutex.Lock()
	if cachedSeriesIds[series.ID] {
		cacheMutex.Unlock()
		return
	}
	cacheMutex.Unlock()

	// Cache poster image
	if series.Image != "" {
		err := cacheImage(series.Image, generateCacheFilename(series.Image, series.ID))
		if err != nil {
			log.Printf("Error caching poster for series %d: %v", series.ID, err)
		}
	}

	// Get and cache background image
	backgroundURL, err := GetSeriesBackground(series.ID)
	if err != nil {
		log.Printf("Error getting background for series %d: %v", series.ID, err)
	} else if backgroundURL != "" {
		err = cacheImage(backgroundURL, fmt.Sprintf("%d-background.jpg", series.ID))
		if err != nil {
			log.Printf("Error caching background for series %d: %v", series.ID, err)
		}
	}

	// Get and cache detailed series info
	_, err = GetSeriesInfo(series.ID)
	if err != nil {
		log.Printf("Error caching series info for series %d: %v", series.ID, err)
	}

	// Mark this series as cached
	cacheMutex.Lock()
	cachedSeriesIds[series.ID] = true
	cacheMutex.Unlock()

	log.Printf("Cached all content for series %d (%s)", series.ID, series.Name)
}

// startBackgroundCaching initiates the caching process for all series content
func startBackgroundCaching() {
	cacheMutex.Lock()
	// Only cache if it's been more than 15 minutes since last cache and no cache is in progress
	if !cacheInProgress && time.Since(lastCacheTime) > 15*time.Minute {
		cacheInProgress = true
		cacheMutex.Unlock()
		
		go func() {
			defer func() {
				cacheMutex.Lock()
				cacheInProgress = false
				lastCacheTime = time.Now()
				cacheMutex.Unlock()
			}()

			log.Printf("Starting background caching process")
			
			// Get all series data
			allSeries, err := GetSeriesData()
			if err != nil {
				log.Printf("Error getting series data for caching: %v", err)
				return
			}

			// Use a WaitGroup to track all goroutines
			var wg sync.WaitGroup

			// Start a goroutine for each series
			for _, series := range allSeries {
				wg.Add(1)
				go cacheSeriesContent(series, &wg)
			}

			// Wait for all caching operations to complete
			wg.Wait()
			log.Printf("Background caching process completed")
		}()
	} else {
		cacheMutex.Unlock()
		if cacheInProgress {
			log.Printf("Background caching already in progress")
		} else {
			log.Printf("Skipping background cache, last cache was less than 15 minutes ago")
		}
	}
}

func setupFrontend(app *fiber.App) {
	// Serve static files (including cached images)
	app.Static("/static", "./static")

	// Main route serves the template and starts background caching
	app.Get("/", func(c *fiber.Ctx) error {
		log.Printf("Serving index page to %s", c.IP())
		// Start background caching after serving the page
		startBackgroundCaching()
		return c.Render("index", fiber.Map{})
	})

	// Route to serve the series detail page
	app.Get("/series/:id", func(c *fiber.Ctx) error {
		log.Printf("Serving series detail page for ID %s to %s", c.Params("id"), c.IP())
		return c.Render("media-detail", fiber.Map{})
	})

	// Route to serve the movie detail page
	app.Get("/movie/:id", func(c *fiber.Ctx) error {
		log.Printf("Serving movie detail page for ID %s to %s", c.Params("id"), c.IP())
		return c.Render("media-detail", fiber.Map{})
	})
}
