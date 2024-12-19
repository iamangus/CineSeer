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
	cachedMediaIds   = make(map[int]bool)
	cacheInProgress  bool
)

// cacheMediaContent handles caching all content for a single media item (movie or series)
func cacheMediaContent(content MediaContent, wg *sync.WaitGroup) {
	defer wg.Done()

	// Check if this content has already been cached
	cacheMutex.Lock()
	if cachedMediaIds[content.ID] {
		cacheMutex.Unlock()
		return
	}
	cacheMutex.Unlock()

	// Cache poster image
	if content.PosterPath != "" {
		err := cacheImage(content.PosterPath, fmt.Sprint(content.ID), "poster")
		if err != nil {
			log.Printf("Error caching poster for content %d: %v", content.ID, err)
		}
	}

	// Cache backdrop image
	if content.BackdropPath != "" {
		err := cacheImage(content.BackdropPath, fmt.Sprint(content.ID), "backdrop")
		if err != nil {
			log.Printf("Error caching backdrop for content %d: %v", content.ID, err)
		}
	}

	// Get and cache detailed info
	var err error
	if isMovie(content) {
		_, err = get_details_movies(content.ID)
	} else {
		_, err = get_details_series(content.ID)
	}
	if err != nil {
		log.Printf("Error caching content info for ID %d: %v", content.ID, err)
	}

	// Mark this content as cached
	cacheMutex.Lock()
	cachedMediaIds[content.ID] = true
	cacheMutex.Unlock()

	log.Printf("Cached all content for ID %d (%s)", content.ID, content.Title)
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
			
			// Get homepage data for caching
			homePageData, err := getHomePageData()
			if err != nil {
				log.Printf("Error getting homepage data for caching: %v", err)
				return
			}

			// Use a WaitGroup to track all goroutines
			var wg sync.WaitGroup

			// Cache trending TV shows
			for _, content := range homePageData.TrendingTV {
				wg.Add(1)
				go cacheMediaContent(content, &wg)
			}

			// Cache trending movies
			for _, content := range homePageData.TrendingMovies {
				wg.Add(1)
				go cacheMediaContent(content, &wg)
			}

			// Cache popular TV shows
			for _, content := range homePageData.PopularTV {
				wg.Add(1)
				go cacheMediaContent(content, &wg)
			}

			// Cache popular movies
			for _, content := range homePageData.PopularMovies {
				wg.Add(1)
				go cacheMediaContent(content, &wg)
			}

			// Cache upcoming movies
			for _, content := range homePageData.UpcomingMovies {
				wg.Add(1)
				go cacheMediaContent(content, &wg)
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
