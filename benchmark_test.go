package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"server/routes"

	"github.com/gin-gonic/gin"
)

// TestServer creates a test server instance
func setupTestServer() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	// Apply the same middleware as main server
	router.Use(CacheMiddleware())
	router.Use(CORSMiddleware())

	// Create API router group (similar to main.go)
	apiRouter := router.Group("/api")

	// Register test routes (without database for benchmarking)
	routes.RegisterTestRoute(apiRouter)
	routes.RegisterStatusRoute(apiRouter)

	// Add some mock endpoints for testing
	apiRouter.GET("/mock-users", func(c *gin.Context) {
		users := []map[string]interface{}{
			{"id": 1, "name": "John Doe", "email": "john@example.com"},
			{"id": 2, "name": "Jane Smith", "email": "jane@example.com"},
			{"id": 3, "name": "Bob Johnson", "email": "bob@example.com"},
		}
		c.JSON(http.StatusOK, gin.H{"users": users, "count": len(users)})
	})

	apiRouter.GET("/mock-events", func(c *gin.Context) {
		events := []map[string]interface{}{
			{"id": 1, "title": "School Assembly", "date": "2024-01-15", "location": "Main Hall"},
			{"id": 2, "title": "Sports Day", "date": "2024-01-20", "location": "Sports Ground"},
			{"id": 3, "title": "Parent Meeting", "date": "2024-01-25", "location": "Conference Room"},
		}
		c.JSON(http.StatusOK, gin.H{"events": events, "count": len(events)})
	})

	apiRouter.POST("/mock-login", func(c *gin.Context) {
		var loginData map[string]interface{}
		if err := c.ShouldBindJSON(&loginData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		// Simulate processing time
		time.Sleep(10 * time.Millisecond)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"token":   "mock-jwt-token-12345",
			"user":    gin.H{"id": 1, "username": "testuser", "role": "student"},
		})
	})

	apiRouter.GET("/heavy-computation", func(c *gin.Context) {
		// Simulate heavy computation
		result := 0
		for i := 0; i < 100000; i++ {
			result += i
		}
		c.JSON(http.StatusOK, gin.H{"result": result, "computed": true})
	})

	return router
}

// Benchmark single endpoint with concurrent requests
func BenchmarkConcurrentRequests(b *testing.B) {
	server := setupTestServer()
	ts := httptest.NewServer(server)
	defer ts.Close()

	b.ResetTimer()

	b.SetParallelism(20)
	b.RunParallel(func(pb *testing.PB) {
		client := &http.Client{Timeout: 30 * time.Second}
		for pb.Next() {
			resp, err := client.Get(ts.URL + "/api/check-status")
			if err != nil {
				b.Errorf("Request failed: %v", err)
				continue
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				b.Errorf("Expected status 200, got %d", resp.StatusCode)
			}
		}
	})
}

// Benchmark multiple endpoints with mixed load
func BenchmarkMixedEndpoints(b *testing.B) {
	server := setupTestServer()
	ts := httptest.NewServer(server)
	defer ts.Close()

	endpoints := []string{
		"/api/check-status",
		"/api/mock-users",
		"/api/mock-events",
		"/api/heavy-computation",
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		client := &http.Client{Timeout: 30 * time.Second}
		endpointIndex := 0

		for pb.Next() {
			endpoint := endpoints[endpointIndex%len(endpoints)]
			endpointIndex++

			resp, err := client.Get(ts.URL + endpoint)
			if err != nil {
				b.Errorf("Request to %s failed: %v", endpoint, err)
				continue
			}
			resp.Body.Close()

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				b.Errorf("Request to %s returned status %d", endpoint, resp.StatusCode)
			}
		}
	})
}

// Benchmark POST requests with JSON payload
func BenchmarkPOSTRequests(b *testing.B) {
	server := setupTestServer()
	ts := httptest.NewServer(server)
	defer ts.Close()

	loginPayload := map[string]string{
		"username": "testuser",
		"password": "testpass123",
	}

	payloadBytes, _ := json.Marshal(loginPayload)

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		client := &http.Client{Timeout: 30 * time.Second}

		for pb.Next() {
			resp, err := client.Post(
				ts.URL+"/api/mock-login",
				"application/json",
				bytes.NewBuffer(payloadBytes),
			)
			if err != nil {
				b.Errorf("POST request failed: %v", err)
				continue
			}
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				b.Errorf("Expected status 200, got %d", resp.StatusCode)
			}
		}
	})
}

// Benchmark with exact 20 simultaneous connections
func BenchmarkExact20Connections(b *testing.B) {
	server := setupTestServer()
	ts := httptest.NewServer(server)
	defer ts.Close()

	const numConnections = 20
	const requestsPerConnection = 10

	b.ResetTimer()

	// Run the benchmark N times
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		results := make(chan bool, numConnections*requestsPerConnection)

		// Start exactly 20 goroutines (connections)
		for conn := 0; conn < numConnections; conn++ {
			wg.Add(1)
			go func(connectionID int) {
				defer wg.Done()
				client := &http.Client{Timeout: 30 * time.Second}

				// Each connection makes 10 requests
				for req := 0; req < requestsPerConnection; req++ {
					// Rotate through different endpoints
					endpoints := []string{
						"/api/check-status",
						"/api/mock-users",
						"/api/mock-events",
						"/api/test",
					}
					endpoint := endpoints[req%len(endpoints)]

					resp, err := client.Get(ts.URL + endpoint)
					success := err == nil && resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 300

					if resp != nil {
						resp.Body.Close()
					}

					results <- success
				}
			}(conn)
		}

		// Wait for all connections to complete
		wg.Wait()
		close(results)

		// Count successful requests
		successCount := 0
		totalCount := 0
		for success := range results {
			totalCount++
			if success {
				successCount++
			}
		}

		if successCount < totalCount {
			b.Logf("Iteration %d: %d/%d requests successful", i+1, successCount, totalCount)
		}
	}
}

// Stress test with increasing load
func BenchmarkStressTest(b *testing.B) {
	server := setupTestServer()
	ts := httptest.NewServer(server)
	defer ts.Close()

	connectionCounts := []int{5, 10, 20, 50}

	for _, connCount := range connectionCounts {
		b.Run(fmt.Sprintf("Connections_%d", connCount), func(b *testing.B) {
			b.SetParallelism(connCount)

			b.RunParallel(func(pb *testing.PB) {
				client := &http.Client{Timeout: 30 * time.Second}

				for pb.Next() {
					resp, err := client.Get(ts.URL + "/api/check-status")
					if err != nil {
						b.Errorf("Request failed with %d connections: %v", connCount, err)
						continue
					}
					resp.Body.Close()

					if resp.StatusCode != http.StatusOK {
						b.Errorf("Expected status 200 with %d connections, got %d", connCount, resp.StatusCode)
					}
				}
			})
		})
	}
}

// Benchmark response time distribution
func BenchmarkResponseTimeDistribution(b *testing.B) {
	server := setupTestServer()
	ts := httptest.NewServer(server)
	defer ts.Close()

	var responseTimes []time.Duration
	var mutex sync.Mutex

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		client := &http.Client{Timeout: 30 * time.Second}

		for pb.Next() {
			start := time.Now()
			resp, err := client.Get(ts.URL + "/api/check-status")
			duration := time.Since(start)

			if err == nil && resp != nil {
				resp.Body.Close()

				mutex.Lock()
				responseTimes = append(responseTimes, duration)
				mutex.Unlock()
			}
		}
	})

	// Calculate statistics
	if len(responseTimes) > 0 {
		var total time.Duration
		min := responseTimes[0]
		max := responseTimes[0]

		for _, rt := range responseTimes {
			total += rt
			if rt < min {
				min = rt
			}
			if rt > max {
				max = rt
			}
		}

		avg := total / time.Duration(len(responseTimes))
		b.Logf("Response time stats - Min: %v, Max: %v, Avg: %v, Samples: %d",
			min, max, avg, len(responseTimes))
	}
}

// Memory allocation benchmark
func BenchmarkMemoryUsage(b *testing.B) {
	server := setupTestServer()
	ts := httptest.NewServer(server)
	defer ts.Close()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		client := &http.Client{Timeout: 30 * time.Second}

		for pb.Next() {
			resp, err := client.Get(ts.URL + "/api/mock-events")
			if err == nil && resp != nil {
				// Read the response body to simulate real usage
				io.ReadAll(resp.Body)
				resp.Body.Close()
			}
		}
	})
}

// Test with different payload sizes
func BenchmarkPayloadSizes(b *testing.B) {
	server := setupTestServer()
	ts := httptest.NewServer(server)
	defer ts.Close()

	// Create payloads of different sizes
	payloadSizes := []int{100, 1000, 10000}

	for _, size := range payloadSizes {
		b.Run(fmt.Sprintf("PayloadSize_%d", size), func(b *testing.B) {
			// Create a payload of specified size
			payload := map[string]interface{}{
				"data": string(make([]byte, size)),
				"test": true,
			}
			payloadBytes, _ := json.Marshal(payload)

			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				client := &http.Client{Timeout: 30 * time.Second}

				for pb.Next() {
					resp, err := client.Post(
						ts.URL+"/api/test",
						"application/json",
						bytes.NewBuffer(payloadBytes),
					)
					if err == nil && resp != nil {
						resp.Body.Close()
					}
				}
			})
		})
	}
}
