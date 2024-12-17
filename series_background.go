package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type ArtworkData struct {
	ID       int    `json:"id"`
	Image    string `json:"image"`
	Language string `json:"language"`
	Type     int    `json:"type"`
}

type ArtworkResponse struct {
	Data struct {
		Artworks []ArtworkData `json:"artworks"`
	} `json:"data"`
}

func GetSeriesBackground(seriesID int) (string, error) {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		return "", fmt.Errorf("error loading .env file: %v", err)
	}

	apiKey := os.Getenv("TVDB_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("TVDB_API_KEY not found in environment")
	}

	// Login to get bearer token
	token, err := login(apiKey)
	if err != nil {
		return "", fmt.Errorf("login failed: %v", err)
	}

	// Make API call using the token
	endpoint := fmt.Sprintf("series/%d/artworks?lang=eng&type=3", seriesID)
	response, err := makeAuthenticatedRequest(token, endpoint)
	if err != nil {
		return "", fmt.Errorf("API request failed: %v", err)
	}

	var artworkResp ArtworkResponse
	if err := json.Unmarshal([]byte(response), &artworkResp); err != nil {
		return "", fmt.Errorf("error parsing JSON: %v", err)
	}

	// Return first background image URL if available
	if len(artworkResp.Data.Artworks) > 0 {
		return artworkResp.Data.Artworks[0].Image, nil
	}

	return "", fmt.Errorf("no background artwork found for series %d", seriesID)
}
