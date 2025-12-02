package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Define constants for the download source and default parameters
const (
	baseURL     = "https://picsum.photos/%d/%d" // Format: /width/height
	outputDir   = "static/images"
	totalImages = 100
	largeCount  = 5
	smallSize   = 300  // 300x300 pixels for small images
	largeSize   = 2400 // 2400x2400 pixels for large images
)

// ImageConfig holds the details for a single image to be downloaded.
type ImageConfig struct {
	ID   int
	URL  string
	Path string
}

func main() {
	fmt.Printf("Starting download of %d images (%d large, %d small).\n", totalImages, largeCount, totalImages-largeCount)
	fmt.Printf("Large images: %dx%d pixels, Small images: %dx%d pixels\n", largeSize, largeSize, smallSize, smallSize)
	fmt.Printf("Saving files to: ./%s/\n", outputDir)

	// Setup output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Error creating output directory %s: %v", outputDir, err)
	}

	// Channel to manage the list of image configurations
	imageJobs := make(chan ImageConfig, totalImages)
	// WaitGroup to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Set a reasonable concurrency limit
	numWorkers := 10
	if totalImages < numWorkers {
		numWorkers = totalImages
	}

	// Start worker goroutines
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker(i, imageJobs, &wg)
	}

	// Queue up the download jobs
	for i := 0; i < totalImages; i++ {
		var imgSize int
		var sizeLabel string

		// First 5 images are large, rest are small
		if i < largeCount {
			imgSize = largeSize
			sizeLabel = "large"
		} else {
			imgSize = smallSize
			sizeLabel = "small"
		}

		filename := fmt.Sprintf("image_%03d_%s_%dx%d.jpg", i+1, sizeLabel, imgSize, imgSize)
		url := fmt.Sprintf(baseURL, imgSize, imgSize)

		// Add a random query parameter to prevent the same image from being cached
		url = fmt.Sprintf("%s?random=%d", url, time.Now().UnixNano()/int64(time.Millisecond)+int64(i))

		imageJobs <- ImageConfig{
			ID:   i + 1,
			URL:  url,
			Path: filepath.Join(outputDir, filename),
		}
	}
	close(imageJobs) // Close the channel to signal workers no more jobs will be added

	// Wait for all workers to complete
	wg.Wait()
	fmt.Println("\nAll downloads finished successfully!")
}

// worker is a goroutine that pulls download jobs from the channel and executes them.
func worker(id int, jobs <-chan ImageConfig, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Printf("Worker %d started.", id)

	// Iterate over the channel until it is closed
	for job := range jobs {
		fmt.Printf("Worker %d: Downloading image %d...\n", id, job.ID)
		if err := downloadFile(job.URL, job.Path); err != nil {
			log.Printf("Worker %d: FAILED to download image %d from %s: %v", id, job.ID, job.URL, err)
		} else {
			fmt.Printf("Worker %d: Successfully saved image %d to %s\n", id, job.ID, job.Path)
		}
	}
	log.Printf("Worker %d finished.", id)
}

// downloadFile performs the actual HTTP request and saves the body to a file.
func downloadFile(url string, filepath string) error {
	// Create a new HTTP client with a reasonable timeout
	client := http.Client{
		Timeout: 30 * time.Second,
	}

	// Create request with context for cancellation/timeouts
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("could not create request: %w", err)
	}

	// Perform the GET request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-200 response status: %s", resp.Status)
	}

	// Create the output file
	out, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("could not create file: %w", err)
	}
	defer out.Close()

	// Use io.Copy to efficiently stream the response body to the file
	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}

	return nil
}
