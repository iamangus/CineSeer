package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Alias struct {
	Language string `json:"language"`
	Name     string `json:"name"`
}

type Status struct {
	ID          int    `json:"id"`
	KeepUpdated bool   `json:"keepUpdated"`
	Name        string `json:"name"`
	RecordType  string `json:"recordType"`
}

type DetailedSeries struct {
	ID               int      `json:"id"`
	Name             string   `json:"name"`
	Aliases          []Alias  `json:"aliases"`
	AverageRuntime   int      `json:"averageRuntime"`
	Country          string   `json:"country"`
	DefaultSeasonType int     `json:"defaultSeasonType"`
	FirstAired       string   `json:"firstAired"`
	Image            string   `json:"image"`
	IsOrderRandomized bool    `json:"isOrderRandomized"`
	LastAired        string   `json:"lastAired"`
	LastUpdated      string   `json:"lastUpdated"`
	NameTranslations []string `json:"nameTranslations"`
	NextAired        string   `json:"nextAired"`
	OriginalCountry  string   `json:"originalCountry"`
	OriginalLanguage string   `json:"originalLanguage"`
	Score            int      `json:"score"`
	Slug             string   `json:"slug"`
	Status           Status   `json:"status"`
	Year             string   `json:"year"`
}

type SeriesAPIResponse struct {
	Data DetailedSeries `json:"data"`
}

func GetSeriesInfo(seriesID int) (*DetailedSeries, error) {
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
	endpoint := fmt.Sprintf("series/%d", seriesID)
	response, err := makeAuthenticatedRequest(token, endpoint)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %v", err)
	}

	var apiResp SeriesAPIResponse
	if err := json.Unmarshal([]byte(response), &apiResp); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	return &apiResp.Data, nil
}
