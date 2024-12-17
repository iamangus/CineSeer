package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const BaseURL = "https://api4.thetvdb.com/v4"

type LoginResponse struct {
	Status  string `json:"status"`
	Data    struct {
		Token string `json:"token"`
	} `json:"data"`
}

func login(apiKey string) (string, error) {
	loginBody := map[string]string{
		"apikey": apiKey,
	}

	jsonBody, err := json.Marshal(loginBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling login body: %v", err)
	}

	req, err := http.NewRequest("POST", BaseURL+"/login", bytes.NewBuffer(jsonBody))
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

	var loginResp LoginResponse
	if err := json.Unmarshal(body, &loginResp); err != nil {
		return "", fmt.Errorf("error unmarshaling response: %v", err)
	}

	return loginResp.Data.Token, nil
}

func makeAuthenticatedRequest(token, endpoint string) (string, error) {
	req, err := http.NewRequest("GET", BaseURL+"/"+endpoint, nil)
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
