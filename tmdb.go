package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"log"
)

var baseURL = "https://api.themoviedb.org/3"
var imageBaseURL = "https://image.tmdb.org/t/p/original"

func getAPIKey() string {
	key := os.Getenv("TMDB_API_KEY")
	if key == "" {
		log.Fatal("TMDB_API_KEY environment variable is not set")
	}
	return key
}

type MediaContent struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Title        string  `json:"title"`
	Overview     string  `json:"overview"`
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	VoteAverage  float64 `json:"vote_average"`
	ReleaseDate  string  `json:"release_date,omitempty"`
	FirstAirDate string  `json:"first_air_date,omitempty"`
	MediaType    string  `json:"media_type"`
}

type TMDBResponse struct {
	Page    int            `json:"page"`
	Results []MediaContent `json:"results"`
}

type SimplifiedSeries struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Image string `json:"image"`
}

type DetailedContent struct {
	ID               int      `json:"id"`
	Name            string   `json:"name"`
	Overview        string   `json:"overview"`
	PosterPath      string   `json:"poster_path"`
	BackdropPath    string   `json:"backdrop_path"`
	VoteAverage     float64  `json:"vote_average"`
	Genres          []Genre  `json:"genres"`
	Tagline         string   `json:"tagline"`
	Status          string   `json:"status"`
	OriginalLanguage string  `json:"original_language"`
	ProductionCountries []ProductionCountry `json:"production_countries"`
	Networks        []Network `json:"networks,omitempty"`
	NumberOfSeasons int      `json:"number_of_seasons,omitempty"`
	Runtime         int      `json:"runtime,omitempty"`
	CreatedBy       []CreatedBy `json:"created_by,omitempty"`
	ReleaseDate     string    `json:"release_date,omitempty"`
	FirstAirDate    string    `json:"first_air_date,omitempty"`
}

type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Network struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ProductionCountry struct {
	ISO31661 string `json:"iso_3166_1"`
	Name     string `json:"name"`
}

type CreatedBy struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func makeRequest(endpoint string) ([]byte, error) {
	client := &http.Client{}
	
	// Get API key at request time
	apiKey := getAPIKey()
	
	// Add API key as query parameter for v3 API
	url := fmt.Sprintf("%s%s?api_key=%s", baseURL, endpoint, apiKey)
	
	// Debug: Print request URL (without API key)
	log.Printf("Making request to: %s%s", baseURL, endpoint)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return nil, err
	}

	req.Header.Add("accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error making request: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return nil, err
	}

	// Check for non-200 status codes and log the response for debugging
	if resp.StatusCode != http.StatusOK {
		log.Printf("TMDB API Error Response (Status %d): %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("TMDB API error: %s (Status: %d)", string(body), resp.StatusCode)
	}

	// Log successful response
	log.Printf("Successful API response for endpoint %s (length: %d bytes)", endpoint, len(body))
	
	// Try to parse the response to check if it's valid JSON
	var jsonCheck interface{}
	if err := json.Unmarshal(body, &jsonCheck); err != nil {
		log.Printf("Warning: Response is not valid JSON: %v", err)
	}

	return body, nil
}

// Image caching functions
func generateCacheFilename(imageURL string, seriesID int) string {
	ext := filepath.Ext(imageURL)
	if ext == "" {
		ext = ".jpg"
	}
	return fmt.Sprintf("%d-poster%s", seriesID, ext)
}

func cacheImage(imageURL string, filename string) error {
	if !filepath.IsAbs(imageURL) {
		imageURL = fmt.Sprintf("%s%s", imageBaseURL, imageURL)
	}

	resp, err := http.Get(imageURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	cachePath := filepath.Join("static", "cache", filename)
	out, err := os.Create(cachePath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// Series-related functions that map to TMDB API
func GetSeriesData() ([]SimplifiedSeries, error) {
	trending, err := get_trending_series()
	if err != nil {
		return nil, err
	}

	var series []SimplifiedSeries
	for _, content := range trending.Results {
		// Get the display name, preferring Name over Title
		displayName := content.Name
		if displayName == "" {
			displayName = content.Title
		}
		
		// Only add if we have a valid name and poster path
		if displayName != "" && content.PosterPath != "" {
			series = append(series, SimplifiedSeries{
				ID:    content.ID,
				Name:  displayName,
				Image: content.PosterPath,
			})
		}
	}
	return series, nil
}

func GetSeriesInfo(seriesID int) (*DetailedContent, error) {
	return get_details_series(seriesID)
}

func GetSeriesBackground(seriesID int) (string, error) {
	details, err := get_details_series(seriesID)
	if err != nil {
		return "", err
	}
	return details.BackdropPath, nil
}

func get_trending_series() (*TMDBResponse, error) {
	data, err := makeRequest("/trending/tv/week")
	if err != nil {
		return nil, err
	}

	var response TMDBResponse
	if err := json.Unmarshal(data, &response); err != nil {
		log.Printf("Error unmarshaling trending series response: %v", err)
		return nil, err
	}

	// Log the first item for debugging
	if len(response.Results) > 0 {
		log.Printf("Sample trending series: ID=%d, Name=%s, Title=%s, PosterPath=%s, FirstAirDate=%s",
			response.Results[0].ID,
			response.Results[0].Name,
			response.Results[0].Title,
			response.Results[0].PosterPath,
			response.Results[0].FirstAirDate)
	}

	return &response, nil
}

func get_popular_series() (*TMDBResponse, error) {
	data, err := makeRequest("/tv/popular")
	if err != nil {
		return nil, err
	}

	var response TMDBResponse
	if err := json.Unmarshal(data, &response); err != nil {
		log.Printf("Error unmarshaling popular series response: %v", err)
		return nil, err
	}

	// Log the first item for debugging
	if len(response.Results) > 0 {
		log.Printf("Sample popular series: ID=%d, Name=%s, Title=%s, PosterPath=%s, FirstAirDate=%s",
			response.Results[0].ID,
			response.Results[0].Name,
			response.Results[0].Title,
			response.Results[0].PosterPath,
			response.Results[0].FirstAirDate)
	}

	return &response, nil
}

func get_recommended_series(seriesID int) (*TMDBResponse, error) {
	data, err := makeRequest(fmt.Sprintf("/tv/%d/recommendations", seriesID))
	if err != nil {
		return nil, err
	}

	var response TMDBResponse
	if err := json.Unmarshal(data, &response); err != nil {
		log.Printf("Error unmarshaling recommended series response: %v", err)
		return nil, err
	}
	return &response, nil
}

func get_trending_movies() (*TMDBResponse, error) {
	data, err := makeRequest("/trending/movie/week")
	if err != nil {
		return nil, err
	}

	var response TMDBResponse
	if err := json.Unmarshal(data, &response); err != nil {
		log.Printf("Error unmarshaling trending movies response: %v", err)
		return nil, err
	}

	// Log the first item for debugging
	if len(response.Results) > 0 {
		log.Printf("Sample trending movie: ID=%d, Name=%s, Title=%s, PosterPath=%s, ReleaseDate=%s",
			response.Results[0].ID,
			response.Results[0].Name,
			response.Results[0].Title,
			response.Results[0].PosterPath,
			response.Results[0].ReleaseDate)
	}

	return &response, nil
}

func get_popular_movies() (*TMDBResponse, error) {
	data, err := makeRequest("/movie/popular")
	if err != nil {
		return nil, err
	}

	var response TMDBResponse
	if err := json.Unmarshal(data, &response); err != nil {
		log.Printf("Error unmarshaling popular movies response: %v", err)
		return nil, err
	}

	// Log the first item for debugging
	if len(response.Results) > 0 {
		log.Printf("Sample popular movie: ID=%d, Name=%s, Title=%s, PosterPath=%s, ReleaseDate=%s",
			response.Results[0].ID,
			response.Results[0].Name,
			response.Results[0].Title,
			response.Results[0].PosterPath,
			response.Results[0].ReleaseDate)
	}

	return &response, nil
}

func get_upcoming_movies() (*TMDBResponse, error) {
	data, err := makeRequest("/movie/upcoming")
	if err != nil {
		return nil, err
	}

	var response TMDBResponse
	if err := json.Unmarshal(data, &response); err != nil {
		log.Printf("Error unmarshaling upcoming movies response: %v", err)
		return nil, err
	}

	// Log the first item for debugging
	if len(response.Results) > 0 {
		log.Printf("Sample upcoming movie: ID=%d, Name=%s, Title=%s, PosterPath=%s, ReleaseDate=%s",
			response.Results[0].ID,
			response.Results[0].Name,
			response.Results[0].Title,
			response.Results[0].PosterPath,
			response.Results[0].ReleaseDate)
	}

	return &response, nil
}

func get_recommended_movies(movieID int) (*TMDBResponse, error) {
	data, err := makeRequest(fmt.Sprintf("/movie/%d/recommendations", movieID))
	if err != nil {
		return nil, err
	}

	var response TMDBResponse
	if err := json.Unmarshal(data, &response); err != nil {
		log.Printf("Error unmarshaling recommended movies response: %v", err)
		return nil, err
	}
	return &response, nil
}

func get_details_series(seriesID int) (*DetailedContent, error) {
	data, err := makeRequest(fmt.Sprintf("/tv/%d", seriesID))
	if err != nil {
		return nil, err
	}

	var response DetailedContent
	if err := json.Unmarshal(data, &response); err != nil {
		log.Printf("Error unmarshaling series details response: %v", err)
		return nil, err
	}
	return &response, nil
}

func get_details_movies(movieID int) (*DetailedContent, error) {
	data, err := makeRequest(fmt.Sprintf("/movie/%d", movieID))
	if err != nil {
		return nil, err
	}

	var response DetailedContent
	if err := json.Unmarshal(data, &response); err != nil {
		log.Printf("Error unmarshaling movie details response: %v", err)
		return nil, err
	}
	return &response, nil
}
