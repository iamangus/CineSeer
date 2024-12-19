package main

import (
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
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

	// Create fiber app
	app := fiber.New()

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
	
	// Setup frontend routes
	setupFrontend(app)

	// Start the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("Starting server on port %s", port)
	log.Fatal(app.Listen(":" + port))
}
