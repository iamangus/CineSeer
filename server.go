package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html/v2"
	"github.com/joho/godotenv"
)

type HomePageData struct {
	TrendingTV        []MediaContent `json:"trending_tv"`
	TrendingMovies    []MediaContent `json:"trending_movies"`
	PopularTV         []MediaContent `json:"popular_tv"`
	PopularMovies     []MediaContent `json:"popular_movies"`
	UpcomingMovies    []MediaContent `json:"upcoming_movies"`
	RecommendedTV     []MediaContent `json:"recommended_tv"`
	RecommendedMovies []MediaContent `json:"recommended_movies"`
}

// Helper function to determine if a MediaContent is a movie
func isMovie(content MediaContent) bool {
	// If it has a release_date, it's a movie
	// If it has a first_air_date, it's a TV show
	return content.ReleaseDate != ""
}

var (
	homePageCache     *HomePageData
	homePageMutex     sync.RWMutex
	lastCacheRefresh  time.Time
	cacheRefreshHours = 3
)

func refreshHomePageCache() error {
	homePageMutex.Lock()
	defer homePageMutex.Unlock()

	newCache := HomePageData{
		TrendingTV:        make([]MediaContent, 0),
		TrendingMovies:    make([]MediaContent, 0),
		PopularTV:         make([]MediaContent, 0),
		PopularMovies:     make([]MediaContent, 0),
		UpcomingMovies:    make([]MediaContent, 0),
		RecommendedTV:     make([]MediaContent, 0),
		RecommendedMovies: make([]MediaContent, 0),
	}

	var wg sync.WaitGroup
	var errChan = make(chan error, 8) // One for each API call

	// Trending TV
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := get_trending_series()
		if err != nil {
			log.Printf("Error fetching trending TV: %v", err)
			errChan <- err
			return
		}
		if resp != nil && len(resp.Results) > 0 {
			log.Printf("Fetched %d trending TV shows", len(resp.Results))
			newCache.TrendingTV = resp.Results
		}
	}()

	// Trending Movies
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := get_trending_movies()
		if err != nil {
			log.Printf("Error fetching trending movies: %v", err)
			errChan <- err
			return
		}
		if resp != nil && len(resp.Results) > 0 {
			log.Printf("Fetched %d trending movies", len(resp.Results))
            newCache.TrendingMovies = resp.Results
        }
	}()

	// Popular TV
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := get_popular_series()
		if err != nil {
			log.Printf("Error fetching popular TV: %v", err)
			errChan <- err
			return
		}
		if resp != nil && len(resp.Results) > 0 {
			log.Printf("Fetched %d popular TV shows", len(resp.Results))
            newCache.PopularTV = resp.Results
        }
	}()

	// Popular Movies
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := get_popular_movies()
		if err != nil {
			log.Printf("Error fetching popular movies: %v", err)
			errChan <- err
			return
		}
		if resp != nil && len(resp.Results) > 0 {
			log.Printf("Fetched %d popular movies", len(resp.Results))
            newCache.PopularMovies = resp.Results
        }
	}()

	// Upcoming Movies
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := get_upcoming_movies()
		if err != nil {
			log.Printf("Error fetching upcoming movies: %v", err)
			errChan <- err
			return
		}
		if resp != nil && len(resp.Results) > 0 {
			log.Printf("Fetched %d upcoming movies", len(resp.Results))
            newCache.UpcomingMovies = resp.Results
        }
	}()

	// Get recommendations based on first popular TV show
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := get_popular_series()
		if err != nil {
			log.Printf("Error fetching popular TV for recommendations: %v", err)
			errChan <- err
			return
		}
		if resp != nil && len(resp.Results) > 0 {
			recResp, err := get_recommended_series(resp.Results[0].ID)
			if err != nil {
				log.Printf("Error fetching TV recommendations: %v", err)
				errChan <- err
				return
			}
			if recResp != nil && len(recResp.Results) > 0 {
				log.Printf("Fetched %d TV recommendations", len(recResp.Results))
                newCache.RecommendedTV = recResp.Results
            }
		}
	}()

	// Get recommendations based on first popular movie
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := get_popular_movies()
		if err != nil {
			log.Printf("Error fetching popular movies for recommendations: %v", err)
			errChan <- err
			return
		}
		if resp != nil && len(resp.Results) > 0 {
			recResp, err := get_recommended_movies(resp.Results[0].ID)
			if err != nil {
				log.Printf("Error fetching movie recommendations: %v", err)
				errChan <- err
				return
			}
			if recResp != nil && len(recResp.Results) > 0 {
				log.Printf("Fetched %d movie recommendations", len(recResp.Results))
                newCache.RecommendedMovies = recResp.Results
            }
		}
	}()

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		if err != nil {
			log.Printf("Error in refreshHomePageCache: %v", err)
			return err
		}
	}

	homePageCache = &newCache
	lastCacheRefresh = time.Now()
	log.Printf("Successfully refreshed home page cache")
	return nil
}

func getHomePageData() (*HomePageData, error) {
	homePageMutex.RLock()
	if homePageCache != nil && time.Since(lastCacheRefresh) < time.Hour*time.Duration(cacheRefreshHours) {
		defer homePageMutex.RUnlock()
		return homePageCache, nil
	}
	homePageMutex.RUnlock()

	if err := refreshHomePageCache(); err != nil {
		log.Printf("Error refreshing home page cache: %v", err)
		return nil, err
	}

	homePageMutex.RLock()
	defer homePageMutex.RUnlock()
	return homePageCache, nil
}

func main() {
	// Load .env file from current directory
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
		// Continue execution as environment variables might be set through other means
	}

	// Verify required environment variables
	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		log.Fatal("TMDB_API_KEY environment variable is required")
	}
	log.Printf("TMDB API Key loaded (length: %d)", len(apiKey))

	// Initialize template engine
	engine := html.New("./views", ".html")

	// Create fiber app
	app := fiber.New(fiber.Config{
		Views: engine,
	})

	// Get base path from environment variable, default to "/"
	basePath := os.Getenv("BASE_PATH")
	if basePath == "" {
		basePath = "/"
	}
	// Ensure base path starts with / and doesn't end with /
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	basePath = strings.TrimSuffix(basePath, "/")
	
	// Setup frontend routes with base path
	app.Static(basePath+"/static", "./static")
	
	// Main route serves the template and starts background caching
	app.Get(basePath+"/", func(c *fiber.Ctx) error {
		log.Printf("Serving index page to %s", c.IP())
		// Start background caching after serving the page
		startBackgroundCaching()
		return c.Render("index", fiber.Map{})
	})

	// Route to serve the series detail page
	app.Get(basePath+"/series/:id", func(c *fiber.Ctx) error {
		log.Printf("Serving series detail page for ID %s to %s", c.Params("id"), c.IP())
		return c.Render("media-detail", fiber.Map{})
	})

	// Route to serve the movie detail page
	app.Get(basePath+"/movie/:id", func(c *fiber.Ctx) error {
		log.Printf("Serving movie detail page for ID %s to %s", c.Params("id"), c.IP())
		return c.Render("media-detail", fiber.Map{})
	})

	// API routes
	api := app.Group(basePath + "/api")

	// Image endpoint
	api.Get("/image/:id/:type", func(c *fiber.Ctx) error {
		contentID := c.Params("id")
		imgType := c.Params("type") // poster or backdrop
		
		if contentID == "" {
			return c.Status(400).JSON(fiber.Map{
				"error": "Content ID is required",
			})
		}

		// Use ID-based filename
		cacheFilename := fmt.Sprintf("%s-%s.jpg", contentID, imgType)
		cachePath := filepath.Join("static", "cache", cacheFilename)

		// Check if image exists in cache
		if _, err := os.Stat(cachePath); os.IsNotExist(err) {
			// Get content details to get image path
			var imagePath string
			id, _ := strconv.Atoi(contentID)

			// Try movie first
			if details, err := get_details_movies(id); err == nil {
				if imgType == "poster" {
					imagePath = details.PosterPath
				} else {
					imagePath = details.BackdropPath
				}
			} else {
				// Try series if movie fails
				if details, err := get_details_series(id); err == nil {
					if imgType == "poster" {
						imagePath = details.PosterPath
					} else {
						imagePath = details.BackdropPath
					}
				} else {
					return c.Status(404).JSON(fiber.Map{
						"error": "Content not found",
					})
				}
			}

			// Download and cache the image
			if err := cacheImage(imagePath, contentID, imgType); err != nil {
				log.Printf("Error caching image: %v", err)
				return c.Status(500).JSON(fiber.Map{
					"error": "Failed to cache image",
				})
			}
		}

		// Serve the cached image
		return c.SendFile(cachePath)
	})

	// Home page data
	api.Get("/home", func(c *fiber.Ctx) error {
		data, err := getHomePageData()
		if err != nil {
			log.Printf("Error getting home page data: %v", err)
			return c.Status(500).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.JSON(data)
	})

	// Series details
	api.Get("/series/:id", func(c *fiber.Ctx) error {
		id, err := c.ParamsInt("id")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error": "Invalid series ID",
			})
		}

		details, err := get_details_series(id)
		if err != nil {
			log.Printf("Error getting series details for ID %d: %v", id, err)
			return c.Status(500).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(details)
	})

	// Movie details
	api.Get("/movie/:id", func(c *fiber.Ctx) error {
		id, err := c.ParamsInt("id")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error": "Invalid movie ID",
			})
		}

		details, err := get_details_movies(id)
		if err != nil {
			log.Printf("Error getting movie details for ID %d: %v", id, err)
			return c.Status(500).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(details)
	})

	// Start the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("Starting server on port %s", port)
	log.Fatal(app.Listen(":" + port))
}
