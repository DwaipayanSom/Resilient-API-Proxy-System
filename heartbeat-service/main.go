package main

import (
	"context"  // for managing background tasks (used with Redis)
	"fmt"      // for formatted printing
	"log"      // for logging info and errors
	"net/http" // to make HTTP requests (used for health check)
	"time"     // to add timeouts, delays, etc.

	"github.com/redis/go-redis/v9" // Redis client library
)

// Declare global variables
var redisClient *redis.Client  // Redis client instance
var ctx = context.Background() // context for Redis operations

func main() {
	// Connect to the Redis service running in Docker
	redisClient = redis.NewClient(&redis.Options{
		Addr: "redis:6379", // Redis hostname inside Docker Compose network
	})

	// Start listening to status messages published on Redis in a new goroutine
	go subscribeToStatus()

	// Continuously check the health of API Proxy every 5 seconds
	for {
		checkHealth()               // perform the health check
		time.Sleep(5 * time.Second) // wait 5 seconds before next check
	}
}

// Makes a GET request to /health endpoint of API Proxy service
func checkHealth() {
	// Create a new HTTP client with a 5-second timeout
	client := http.Client{
		Timeout: 5 * time.Second,
	}

	// Attempt to call the health endpoint of the api-proxy service
	resp, err := client.Get("http://api-proxy:8080/health") // Use Docker Compose service name
	if err != nil {
		// If there's a network error (e.g., container not reachable)
		log.Println("❌ API Proxy is unreachable:", err)
		mockSlackAlert("API Proxy failed health check!") // send mock alert
		return
	}
	defer resp.Body.Close() // close the response body when done

	// If the response doesn't return HTTP 200 OK
	if resp.StatusCode != 200 {
		log.Println("❌ API Proxy unhealthy status code:", resp.StatusCode)
		mockSlackAlert("API Proxy returned non-200 from /health!") // send mock alert
		return
	}

	// If everything is fine
	log.Println("✅ API Proxy is healthy")
}

// Listens for messages published on the Redis channel and prints them
func subscribeToStatus() {
	// Subscribe to the "status_channel" in Redis
	sub := redisClient.Subscribe(ctx, "status_channel")

	// Get the channel that receives published messages
	ch := sub.Channel()

	// Loop over incoming messages
	for msg := range ch {
		log.Println("📡 Status from API Proxy:", msg.Payload) // print message content to console
	}
}

// Simulates sending an alert to Slack (just prints to terminal)
func mockSlackAlert(msg string) {
	fmt.Println("🚨 MOCK SLACK ALERT:", msg)
}
