package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	CacheDir     = "static/cache"
	CacheBaseURL = "/static/cache"
)

// getCachedImageURL returns the local URL for a cached image
// If the image isn't cached, it downloads and caches it first
func getCachedImageURL(originalURL string) (string, error) {
	if originalURL == "" {
		return "", fmt.Errorf("empty image URL")
	}

	// Generate a unique filename based on the URL
	filename := generateCacheFilename(originalURL)
	cachePath := filepath.Join(CacheDir, filename)

	// Check if the file exists in cache
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		// File doesn't exist, download it
		if err := downloadAndCacheImage(originalURL, cachePath); err != nil {
			return "", fmt.Errorf("failed to cache image: %v", err)
		}
	}

	// Return the local URL for the cached image
	return fmt.Sprintf("%s/%s", CacheBaseURL, filename), nil
}

// generateCacheFilename creates a unique filename for the cached image
func generateCacheFilename(url string) string {
	// Extract file extension from URL
	ext := filepath.Ext(url)
	if ext == "" {
		ext = ".jpg" // Default to .jpg if no extension found
	}

	// Generate hash of URL for unique filename
	hash := sha256.Sum256([]byte(url))
	return fmt.Sprintf("%x%s", hash[:8], ext) // Use first 8 bytes of hash
}

// downloadAndCacheImage downloads an image from the URL and saves it to the cache
func downloadAndCacheImage(url, cachePath string) error {
	// Create HTTP client with timeout
	client := &http.Client{}

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	// Add User-Agent header to avoid potential 403 errors
	req.Header.Set("User-Agent", "Mozilla/5.0")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error downloading image: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download image, status: %d", resp.StatusCode)
	}

	// Verify content type is an image
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		return fmt.Errorf("invalid content type: %s", contentType)
	}

	// Create cache file
	out, err := os.Create(cachePath)
	if err != nil {
		return fmt.Errorf("error creating cache file: %v", err)
	}
	defer out.Close()

	// Copy the response body to the cache file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		// Clean up the partially written file
		os.Remove(cachePath)
		return fmt.Errorf("error writing cache file: %v", err)
	}

	log.Printf("Successfully cached image from %s to %s", url, cachePath)
	return nil
}

// transformImageURLs modifies the Image URLs in a slice of series to use cached versions
func transformImageURLs[T any](series []T, getImageURL func(T) string, setImageURL func(T, string) T) ([]T, error) {
	transformed := make([]T, len(series))
	for i, s := range series {
		originalURL := getImageURL(s)
		if originalURL != "" {
			cachedURL, err := getCachedImageURL(originalURL)
			if err != nil {
				log.Printf("Warning: Failed to cache image for series: %v", err)
				// Use original URL as fallback
				transformed[i] = s
				continue
			}
			transformed[i] = setImageURL(s, cachedURL)
		} else {
			transformed[i] = s
		}
	}
	return transformed, nil
}
