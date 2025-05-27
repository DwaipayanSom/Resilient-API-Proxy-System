package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"context"

	"github.com/redis/go-redis/v9"
)

var (
	activeAPI      = "openweathermap"
	inactiveAPIs   = map[string]bool{"openweathermap": false, "wttr": false}
	openWeatherKey = os.Getenv("OPENWEATHER_API_KEY")
	redisClient    *redis.Client
)

func main() {
	// Connect to Redis
	redisClient = redis.NewClient(&redis.Options{
		Addr: "redis:6379", // Redis container hostname in Docker Compose
	})
	defer redisClient.Close()

	http.HandleFunc("/weather", weatherHandler)
	http.HandleFunc("/health", healthHandler)

	log.Println("API Proxy running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// Simple health check endpoint
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("alive"))
}

// Weather handler
func weatherHandler(w http.ResponseWriter, r *http.Request) {
	city := r.URL.Query().Get("city")
	if city == "" {
		http.Error(w, "Missing ?city= parameter", http.StatusBadRequest)
		return
	}

	data, err := getWeatherData(city)
	if err != nil {
		// Fallback stub
		response := map[string]string{
			"weather": "unavailable",
			"note":    "all providers failed, returning stubbed response",
		}
		json.NewEncoder(w).Encode(response)
		publishStatus("FALLBACK: Stub response due to total failure")
		return
	}

	json.NewEncoder(w).Encode(data)
}

var (
	// Circuit breaker tracking
	circuitState    = map[string]string{"openweathermap": "closed", "wttr": "closed"}
	failureCount    = map[string]int{"openweathermap": 0, "wttr": 0}
	lastFailureTime = map[string]time.Time{"openweathermap": time.Time{}, "wttr": time.Time{}}
)

func getWeatherData(city string) (map[string]interface{}, error) {

	apis := []string{"openweathermap", "wttr"}

	for _, api := range apis {
		if inactiveAPIs[api] {
			continue
		}

		// Circuit breaker check
		state := circuitState[api]
		if state == "open" {
			// Check if cooldown passed
			if time.Since(lastFailureTime[api]) < 30*time.Second {
				log.Printf("⛔ Circuit open for %s — skipping", api)
				continue
			}
			log.Printf("🔄 Circuit half-open for %s — retrying...", api)
			circuitState[api] = "half-open"
		}

		var url string
		if api == "openweathermap" {
			url = fmt.Sprintf("https://api.openweathermap.org/data/2.5/weather?q=%s&appid=%s", city, openWeatherKey)
		} else {
			url = fmt.Sprintf("https://wttr.in/%s?format=j1", city)
		}

		client := http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(url)

		if err != nil || resp.StatusCode != 200 {
			log.Printf("⚠️ API %s failed: %v", api, err)
			failureCount[api]++
			lastFailureTime[api] = time.Now()

			if failureCount[api] >= 3 {
				circuitState[api] = "open"
				publishStatus(fmt.Sprintf("🚫 Circuit opened for %s after 3 failures", api))
			} else {
				publishStatus(fmt.Sprintf("⚠️ Failure %d for %s", failureCount[api], api))
			}

			continue
		}

		defer resp.Body.Close()

		// Reset on success
		failureCount[api] = 0
		circuitState[api] = "closed"
		publishStatus(fmt.Sprintf("✅ Circuit closed for %s — success!", api))

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		publishStatus(fmt.Sprintf("✅ Success from %s", api))
		return result, nil
	}

	return nil, fmt.Errorf("all APIs failed")
}

func publishStatus(message string) {
	ctx := context.Background()
	err := redisClient.Publish(ctx, "status_channel", message).Err()
	if err != nil {
		log.Println("Error publishing to Redis:", err)
	}
}
