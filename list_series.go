package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	// Change this endpoint to query different resources
	Endpoint = "series/filter?country=usa&lang=eng&sort=firstAired&sortType=desc"
)

var MinScore = 2000 // Minimum score threshold for displaying series

type APIResponse struct {
	Data []Series `json:"data"`
}

type Series struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Year       string `json:"year"`
	Overview   string `json:"overview"`
	Score      int    `json:"score"`
	Image      string `json:"image"`
	FirstAired string `json:"firstAired"`
}

type SimplifiedSeries struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Year       string `json:"year"`
	Score      int    `json:"score"`
	Overview   string `json:"overview"`
	Image      string `json:"image"`
	FirstAired string `json:"firstAired"`
}

func GetSeriesData() ([]SimplifiedSeries, error) {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		return nil, fmt.Errorf("error loading .env file: %v", err)
	}

	apiKey := os.Getenv("TVDB_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("TVDB_API_KEY not found in environment")
	}

	// Login to get bearer token
	token, err := login(apiKey)
	if err != nil {
		return nil, fmt.Errorf("login failed: %v", err)
	}

	// Make API call using the token
	response, err := makeAuthenticatedRequest(token, Endpoint)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %v", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal([]byte(response), &apiResp); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	// Filter series based on MinScore only, not on FirstAired date
	var simplified []SimplifiedSeries
	for _, series := range apiResp.Data {
		// Check for valid image URL (not empty and starts with https://)
		hasValidImage := series.Image != "" && strings.HasPrefix(series.Image, "https://")
		if !hasValidImage {
			log.Printf("Skipping series '%s' due to invalid image URL: %s", series.Name, series.Image)
			continue
		}

		// Only filter by score, not by date
		if series.Score >= MinScore {
			log.Printf("Adding series '%s' with image URL: %s", series.Name, series.Image)
			simplified = append(simplified, SimplifiedSeries{
				ID:         series.ID,
				Name:       series.Name,
				Year:       series.Year,
				Score:      series.Score,
				Overview:   series.Overview,
				Image:      series.Image,
				FirstAired: series.FirstAired,
			})
		}
	}

	// Sort simplified series by FirstAired (newest to oldest)
	sort.Slice(simplified, func(i, j int) bool {
		date1, _ := time.Parse("2006-01-02", simplified[i].FirstAired)
		date2, _ := time.Parse("2006-01-02", simplified[j].FirstAired)
		return date2.Before(date1)
	})

	log.Printf("Returning %d series with valid images", len(simplified))
	return simplified, nil
}
