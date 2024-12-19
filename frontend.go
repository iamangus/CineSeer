package main

import (
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/gofiber/fiber/v2"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	cacheMutex       sync.Mutex
	lastCacheTime    time.Time
	cachedMediaIds   = make(map[int]bool)
	cacheInProgress  bool
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

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

	// Serve static files (including cached images)
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
		return c.Render("media-detail", fiber.Map{
			"Path": fmt.Sprintf("api/content/series/%s", c.Params("id")),
		})
	})

	// Route to serve the movie detail page
	app.Get(basePath+"/movie/:id", func(c *fiber.Ctx) error {
		log.Printf("Serving movie detail page for ID %s to %s", c.Params("id"), c.IP())
		return c.Render("media-detail", fiber.Map{
			"Path": fmt.Sprintf("api/content/movie/%s", c.Params("id")),
		})
	})

	// API routes
	api := app.Group(basePath + "/api")

	// Home page data endpoint with HTML rendering
	api.Get("/home", func(c *fiber.Ctx) error {
		mediaType := c.Query("type")
		if mediaType == "" {
			return c.Status(400).JSON(fiber.Map{
				"error": "Media type is required",
			})
		}

		homeData, err := getHomePageData()
		if err != nil {
			log.Printf("Error getting home page data: %v", err)
			return c.Status(500).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		var items []MediaContent
		switch mediaType {
		case "trending_tv":
			items = homeData.TrendingTV
		case "trending_movies":
			items = homeData.TrendingMovies
		case "popular_tv":
			items = homeData.PopularTV
		case "popular_movies":
			items = homeData.PopularMovies
		case "upcoming_movies":
			items = homeData.UpcomingMovies
		case "recommended_tv":
			items = homeData.RecommendedTV
		case "recommended_movies":
			items = homeData.RecommendedMovies
		default:
			return c.Status(400).SendString("<div class='error'>Invalid media type</div>")
		}

		if len(items) == 0 {
			return c.SendString("<div class='error'>No content available</div>")
		}

		// Build HTML for valid items
		var html strings.Builder
		for _, item := range items[:min(len(items), 20)] {
			if item.Title == "" {
				item.Title = item.Name
			}
			if item.Title != "" && item.PosterPath != "" {
				var year string
				var contentType string
				if item.ReleaseDate != "" {
					if t, err := time.Parse("2006-01-02", item.ReleaseDate); err == nil {
						year = fmt.Sprint(t.Year())
					}
					contentType = "movie"
				} else if item.FirstAirDate != "" {
					if t, err := time.Parse("2006-01-02", item.FirstAirDate); err == nil {
						year = fmt.Sprint(t.Year())
					}
					contentType = "series"
				}

				var buf strings.Builder
				if err := c.App().Config().Views.Render(&buf, "media-card", fiber.Map{
					"ID": item.ID,
					"Title": item.Title,
					"Year": year,
					"Overview": item.Overview,
					"Type": contentType,
				}); err != nil {
					log.Printf("Error rendering media card: %v", err)
					continue
				}
				html.WriteString(buf.String())
			}
		}

		if html.Len() == 0 {
			return c.SendString("<div class='error'>No valid content available</div>")
		}

		return c.SendString(html.String())
	})

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

	// Content details with HTML rendering
	api.Get("/content/series/:id", func(c *fiber.Ctx) error {
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

		return renderMediaContent(c, details, "series", basePath)
	})

	api.Get("/content/movie/:id", func(c *fiber.Ctx) error {
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

		return renderMediaContent(c, details, "movie", basePath)
	})

	// Season details endpoint
	api.Get("/content/series/:id/season/:season", func(c *fiber.Ctx) error {
		id, err := c.ParamsInt("id")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error": "Invalid series ID",
			})
		}

		season, err := c.ParamsInt("season")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error": "Invalid season number",
			})
		}

		details, err := get_season_details(id, season)
		if err != nil {
			log.Printf("Error getting season details for series %d season %d: %v", id, season, err)
			return c.Status(500).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		// Format episodes list
		var episodes strings.Builder
		for _, ep := range details.Episodes {
			episodes.WriteString(fmt.Sprintf(`
				<div class="episode">
					<div class="episode-number">Episode %d</div>
					<div class="episode-title">%s</div>
					<div class="episode-overview">%s</div>
					<div class="episode-meta">
						Air Date: %s | Rating: %.1f/10 (%d votes)
					</div>
				</div>
			`, ep.EpisodeNumber, ep.Name, ep.Overview, ep.AirDate, ep.VoteAverage, ep.VoteCount))
		}

		return c.SendString(episodes.String())
	})
}

func renderMediaContent(c *fiber.Ctx, content *DetailedContent, contentType string, basePath string) error {
	// Get the title, preferring Title over Name
	title := content.Title
	if title == "" {
		title = content.Name
	}

	// Format release date and year
	var releaseDate, year string
	if content.ReleaseDate != "" {
		if t, err := time.Parse("2006-01-02", content.ReleaseDate); err == nil {
			releaseDate = t.Format("January 2, 2006")
			year = fmt.Sprint(t.Year())
		}
	} else if content.FirstAirDate != "" {
		if t, err := time.Parse("2006-01-02", content.FirstAirDate); err == nil {
			releaseDate = t.Format("January 2, 2006")
			year = fmt.Sprint(t.Year())
		}
	}

	// Format data for template
	data := fiber.Map{
		"id":    content.ID,
		"title": title,
		"year":  year,
		"duration": func() string {
			if contentType == "movie" {
				return fmt.Sprintf("%d minutes", content.Runtime)
			}
			return fmt.Sprintf("%d Season%s", content.NumberOfSeasons, map[bool]string{true: "s"}[content.NumberOfSeasons != 1])
		}(),
		"status":     content.Status,
		"genres":     strings.Join(func() []string { genres := make([]string, len(content.Genres)); for i, g := range content.Genres { genres[i] = g.Name }; return genres }(), ", "),
		"genreTags":  strings.Join(func() []string { tags := make([]string, len(content.Genres)); for i, g := range content.Genres { tags[i] = fmt.Sprintf(`<span class="genre-tag">%s</span>`, g.Name) }; return tags }(), ""),
		"tagline":    content.Tagline,
		"overview":   content.Overview,
		"collection": content.BelongsToCollection,
		"seasons": func() string {
			if contentType != "series" {
				return ""
			}
			seasons := make([]string, content.NumberOfSeasons)
			for i := range seasons {
				seasons[i] = fmt.Sprintf(`
					<div class="season">
						<div class="season-header" hx-get="../api/content/series/%d/season/%d" hx-target="#season-%d">
							Season %d
						</div>
						<div class="season-content" id="season-%d">
							Loading season details...
						</div>
					</div>
				`, content.ID, i+1, i+1, i+1, i+1)
			}
			return strings.Join(seasons, "")
		}(),
		"voteAverage": int(content.VoteAverage * 10),
		"popularity":  int(content.Popularity),
		"voteCount":  content.VoteCount,
		"rating":     fmt.Sprintf("%.1f", content.VoteAverage),
		"revenue":    humanize.Comma(content.Revenue),
		"budget":     humanize.Comma(content.Budget),
		"language":   strings.ToUpper(content.OriginalLanguage),
		"countries":  strings.Join(func() []string { countries := make([]string, len(content.ProductionCountries)); for i, c := range content.ProductionCountries { countries[i] = c.Name }; return countries }(), ", "),
		"studios":    strings.Join(func() []string { studios := make([]string, len(content.ProductionCompanies)); for i, s := range content.ProductionCompanies { studios[i] = s.Name }; return studios }(), ", "),
		"director":   func() string { for _, c := range content.Credits.Crew { if c.Job == "Director" { return c.Name } }; return "N/A" }(),
		"screenplay": func() string { for _, c := range content.Credits.Crew { if c.Job == "Screenplay" { return c.Name } }; return "N/A" }(),
		"producer":   strings.Join(func() []string { producers := []string{}; for _, c := range content.Credits.Crew { if c.Job == "Producer" { producers = append(producers, c.Name) } }; return producers }(), ", "),
		"keywords":   strings.Join(func() []string { keywords := make([]string, len(content.Keywords.Keywords)); for i, k := range content.Keywords.Keywords { keywords[i] = fmt.Sprintf(`<span class="genre-tag">%s</span>`, k.Name) }; return keywords }(), ""),
		"basePath":   basePath,
	}

	// Set backdrop path
	if content.BackdropPath != "" {
		data["backdrop"] = fmt.Sprintf("../api/image/%d/backdrop", content.ID)
	}

	// Add release date to data
	data["releaseDate"] = releaseDate

	log.Printf("Rendering content template for %s %d", contentType, content.ID)
	return c.Render("content-template", data)
}
