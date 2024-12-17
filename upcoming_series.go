package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	UpcomingBaseURL = "https://api4.thetvdb.com/v4"
	// Change this endpoint to query different resources
	UpcomingEndpoint = "series/filter?country=usa&lang=eng&sort=firstAired&sortType=desc"
)

var UpcomingMinScore = 500 // Minimum score threshold for displaying series

type UpcomingLoginResponse struct {
	Status  string          `json:"status"`
	Data    UpcomingData   `json:"data"`
	Message string         `json:"message"`
}

type UpcomingData struct {
	Token string `json:"token"`
}

type UpcomingAPIResponse struct {
	Data []UpcomingSeries `json:"data"`
}

type UpcomingSeries struct {
	Name       string `json:"name"`
	Year       string `json:"year"`
	Overview   string `json:"overview"`
	Score      int    `json:"score"`
	Image      string `json:"image"`
	FirstAired string `json:"firstAired"`
}

type SimplifiedUpcomingSeries struct {
	Name       string `json:"name"`
	Year       string `json:"year"`
	Score      int    `json:"score"`
	Overview   string `json:"overview"`
	Image      string `json:"image"`
	FirstAired string `json:"firstAired"`
}

func GetUpcomingSeriesData() ([]SimplifiedUpcomingSeries, error) {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		return nil, fmt.Errorf("error loading .env file: %v", err)
	}

	apiKey := os.Getenv("TVDB_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("TVDB_API_KEY not found in environment")
	}

	// Login to get bearer token
	token, err := loginUpcoming(apiKey)
	if err != nil {
		return nil, fmt.Errorf("login failed: %v", err)
	}

	// Make API call using the token
	response, err := makeUpcomingAuthenticatedRequest(token, UpcomingEndpoint)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %v", err)
	}

	var apiResp UpcomingAPIResponse
	if err := json.Unmarshal([]byte(response), &apiResp); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}

	// Filter series based on MinScore, FirstAired date, and valid image URL
	var simplified []SimplifiedUpcomingSeries
	today := time.Now()
	for _, series := range apiResp.Data {
		firstAired, err := time.Parse("2006-01-02", series.FirstAired)
		if err != nil {
			log.Printf("Skipping series '%s' due to invalid date: %s", series.Name, series.FirstAired)
			continue // Skip series with invalid dates
		}
		
		// Check for valid image URL (not empty and starts with https://)
		hasValidImage := series.Image != "" && strings.HasPrefix(series.Image, "https://")
		if !hasValidImage {
			log.Printf("Skipping series '%s' due to invalid image URL: %s", series.Name, series.Image)
			continue
		}
		
		// Only include series with FirstAired date after today, valid score, and valid image
		if series.Score >= UpcomingMinScore && firstAired.After(today) {
			log.Printf("Adding upcoming series '%s' with image URL: %s", series.Name, series.Image)
			simplified = append(simplified, SimplifiedUpcomingSeries{
				Name:       series.Name,
				Year:       series.Year,
				Score:      series.Score,
				Overview:   series.Overview,
				Image:      series.Image,
				FirstAired: series.FirstAired,
			})
		}
	}

	// Sort simplified series by FirstAired (soonest to latest)
	sort.Slice(simplified, func(i, j int) bool {
		date1, _ := time.Parse("2006-01-02", simplified[i].FirstAired)
		date2, _ := time.Parse("2006-01-02", simplified[j].FirstAired)
		return date1.Before(date2)
	})

	log.Printf("Returning %d upcoming series with valid images", len(simplified))
	return simplified, nil
}

func loginUpcoming(apiKey string) (string, error) {
	loginBody := map[string]string{
		"apikey": apiKey,
	}

	jsonBody, err := json.Marshal(loginBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling login body: %v", err)
	}

	req, err := http.NewRequest("POST", UpcomingBaseURL+"/login", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login failed with status %d: %s", resp.StatusCode, string(body))
	}

	var loginResp UpcomingLoginResponse
	if err := json.Unmarshal(body, &loginResp); err != nil {
		return "", fmt.Errorf("error unmarshaling response: %v", err)
	}

	return loginResp.Data.Token, nil
}

func makeUpcomingAuthenticatedRequest(token, endpoint string) (string, error) {
	req, err := http.NewRequest("GET", UpcomingBaseURL+"/"+endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return string(body), nil
}
