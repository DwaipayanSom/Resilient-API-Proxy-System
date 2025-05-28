package main

import (
	"context"       // used for managing cancellation, deadlines (used with Redis)
	"encoding/json" // for converting Go data to JSON and vice versa
	"fmt"           // for formatted I/O like Printf, Sprintf, etc.
	"log"           // for logging information and errors
	"net/http"      // for building HTTP servers and clients
	"os"            // for interacting with the environment, like getting env variables
	"time"          // for time operations like delays, timeouts, timestamps

	"github.com/redis/go-redis/v9" // Redis client package
)

// Declare global variables
var (
	activeAPI      = "openweathermap"                                        // currently preferred API provider
	inactiveAPIs   = map[string]bool{"openweathermap": false, "wttr": false} // keep track of disabled APIs
	openWeatherKey = os.Getenv("OPENWEATHER_API_KEY")                        // get API key from environment variable
	redisClient    *redis.Client                                             // Redis client instance (will be initialized later)
)

func main() {
	// Connect to the Redis server running in the 'redis' container (via Docker Compose)
	redisClient = redis.NewClient(&redis.Options{
		Addr: "redis:6379", // Redis hostname and port inside the Docker network
	})
	defer redisClient.Close() // ensure connection is closed when the program exits

	// Register HTTP route handlers
	http.HandleFunc("/weather", weatherHandler) // handles requests to get weather
	http.HandleFunc("/health", healthHandler)   // handles health check requests

	log.Println("API Proxy running on :8080")    // print a message to the console
	log.Fatal(http.ListenAndServe(":8080", nil)) // start the HTTP server on port 8080
}

// Basic health check endpoint to confirm service is alive
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK) // send HTTP 200 OK status
	w.Write([]byte("alive"))     // write "alive" as response body
}

// Handles incoming weather API requests
func weatherHandler(w http.ResponseWriter, r *http.Request) {
	// Read the 'city' query parameter from the request URL (e.g., ?city=London)
	city := r.URL.Query().Get("city")
	if city == "" {
		http.Error(w, "Missing ?city= parameter", http.StatusBadRequest)
		return // exit early if city parameter is missing
	}

	// Fetch weather data using the `getWeatherData` function
	data, err := getWeatherData(city)
	if err != nil {
		// If all providers fail, return a dummy fallback response
		response := map[string]string{
			"weather": "unavailable",
			"note":    "all providers failed, returning stubbed response",
		}
		json.NewEncoder(w).Encode(response)                           // send JSON fallback response
		publishStatus("FALLBACK: Stub response due to total failure") // publish status to Redis
		return
	}

	// Send the actual weather data as a JSON response
	json.NewEncoder(w).Encode(data)
}

// Variables to track circuit breaker state for each provider
var (
	circuitState    = map[string]string{"openweathermap": "closed", "wttr": "closed"}          // current state: open, closed, half-open
	failureCount    = map[string]int{"openweathermap": 0, "wttr": 0}                           // failure count for each API
	lastFailureTime = map[string]time.Time{"openweathermap": time.Time{}, "wttr": time.Time{}} // last time each API failed
)

// Tries to get weather data from multiple providers with circuit breaker logic
func getWeatherData(city string) (map[string]interface{}, error) {

	apis := []string{"openweathermap", "wttr"} // list of APIs to try in order

	for _, api := range apis {
		// Skip inactive APIs
		if inactiveAPIs[api] {
			continue
		}

		// Check if circuit is open (i.e., temporarily blocking this API)
		state := circuitState[api]
		if state == "open" {
			// If still within cooldown period, skip this API
			if time.Since(lastFailureTime[api]) < 30*time.Second {
				log.Printf("⛔ Circuit open for %s — skipping", api)
				continue
			}
			// After cooldown, attempt a retry in half-open state
			log.Printf("🔄 Circuit half-open for %s — retrying...", api)
			circuitState[api] = "half-open"
		}

		// Build the API request URL depending on provider
		var url string
		if api == "openweathermap" {
			url = fmt.Sprintf("https://api.openweathermap.org/data/2.5/weather?q=%s&appid=%s", city, openWeatherKey)
		} else {
			url = fmt.Sprintf("https://wttr.in/%s?format=j1", city)
		}

		// Create a new HTTP client with timeout
		client := http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(url) // send the GET request

		// Handle failed request or non-200 status code
		if err != nil || resp.StatusCode != 200 {
			log.Printf("⚠️ API %s failed: %v", api, err)
			failureCount[api]++               // increase failure count
			lastFailureTime[api] = time.Now() // record when the failure happened

			// If this API fails 3 times in a row, open the circuit
			if failureCount[api] >= 3 {
				circuitState[api] = "open"
				publishStatus(fmt.Sprintf("🚫 Circuit opened for %s after 3 failures", api))
			} else {
				// Otherwise just log the failure attempt
				publishStatus(fmt.Sprintf("⚠️ Failure %d for %s", failureCount[api], api))
			}

			continue // try the next API in the list
		}

		defer resp.Body.Close() // ensure response body gets closed

		// On successful response: reset failure tracking
		failureCount[api] = 0
		circuitState[api] = "closed"
		publishStatus(fmt.Sprintf("✅ Circuit closed for %s — success!", api))

		// Decode JSON body into a generic map
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		publishStatus(fmt.Sprintf("✅ Success from %s", api))
		return result, nil // return successful data
	}

	// If all APIs failed, return an error
	return nil, fmt.Errorf("all APIs failed")
}

// Publishes a status update message to Redis channel for others to subscribe
func publishStatus(message string) {
	ctx := context.Background()                                      // create a background context
	err := redisClient.Publish(ctx, "status_channel", message).Err() // publish to "status_channel"
	if err != nil {
		log.Println("Error publishing to Redis:", err) // log any publish failure
	}
}
