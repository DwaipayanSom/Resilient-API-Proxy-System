# Resilient API Proxy with Dockerized Microservices

## 🌐 Overview

This project is a Dockerized microservices system simulating real-world API reliability issues and fault-tolerance mechanisms.

It includes:
- **API Proxy Service** — exposes an endpoint to fetch weather-like data from two external APIs, with fallback and circuit breaker logic.
- **Heartbeat Service** — continuously checks health of the API Proxy and logs/report failures.
- **Redis** — provides pub/sub communication for real-time updates between services.

---

## ⚙️ System Architecture

- `api-proxy`: Exposes `/weather?city=CityName` and `/health`. Handles:
  - Fallback logic between two unreliable APIs
  - Retry on temporary errors
  - Circuit breaker on persistent failures
  - Publishes real-time API status to Redis

- `heartbeat-service`: 
  - Pings `/health` every 5 seconds
  - Logs failures if no response within timeout
  - Sends alert messages back to API Proxy via Redis Pub/Sub

- `redis`: Used for two-way real-time communication between services.

---

## 🏗️ Features Implemented

### ✅ Core Features
- [x] Resilient API proxy with:
  - API fallback
  - Error classification (temporary vs. permanent)
  - Rate limiting handling
- [x] Heartbeat monitoring every 5 seconds
- [x] Real-time Redis-based pub/sub communication

### ✨ Enhancements
- [x] Circuit breaker pattern:
  - Opens after 3 failures
  - 30s cooldown
  - Half-open retry logic
- [x] Stubbed response if both APIs fail
- [x] Logs and color-coded console output for readability

---

## 🚀 Usage

### 🔧 Prerequisites

- Docker + Docker Compose
- Redis image will be pulled automatically

### 🔨 Run Locally

```bash
# Clone this repository
git clone https://github.com/DwaipayanSom/Resilient-API-Proxy-System.git
cd resilient-api-proxy

# Start the system
docker-compose up --build
