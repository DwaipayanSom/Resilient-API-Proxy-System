package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

var redisClient *redis.Client
var ctx = context.Background()

func main() {
	// Connect to Redis
	redisClient = redis.NewClient(&redis.Options{
		Addr: "redis:6379", // Redis host in Docker Compose
	})

	// Start Redis subscriber in a goroutine
	go subscribeToStatus()

	// Run heartbeat every 5 seconds
	for {
		checkHealth()
		time.Sleep(5 * time.Second)
	}
}

// Check API Proxy /health endpoint
func checkHealth() {
	client := http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get("http://api-proxy:8080/health") // Docker Compose service name
	if err != nil {
		log.Println("❌ API Proxy is unreachable:", err)
		mockSlackAlert("API Proxy failed health check!")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Println("❌ API Proxy unhealthy status code:", resp.StatusCode)
		mockSlackAlert("API Proxy returned non-200 from /health!")
		return
	}

	log.Println("✅ API Proxy is healthy")
}

// Subscribe to Redis status channel
func subscribeToStatus() {
	sub := redisClient.Subscribe(ctx, "status_channel")
	ch := sub.Channel()

	for msg := range ch {
		log.Println("📡 Status from API Proxy:", msg.Payload)
	}
}

// Mock alert (console log)
func mockSlackAlert(msg string) {
	fmt.Println("🚨 MOCK SLACK ALERT:", msg)
}
