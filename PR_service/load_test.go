package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	fmt.Println("🚀 Quick load test for PR Service...")

	// Ждем немного чтобы сервер точно запустился
	time.Sleep(2 * time.Second)

	tests := []struct {
		name   string
		method string
		path   string
		body   interface{}
	}{
		{"Create Team 1", "POST", "/team/add", map[string]interface{}{
			"team_name": "load-team-1",
			"members": []map[string]interface{}{
				{"user_id": "load-u1", "username": "Load User 1", "is_active": true},
				{"user_id": "load-u2", "username": "Load User 2", "is_active": true},
				{"user_id": "load-u3", "username": "Load User 3", "is_active": true},
			},
		}},

		{"Create Team 2", "POST", "/team/add", map[string]interface{}{
			"team_name": "load-team-2",
			"members": []map[string]interface{}{
				{"user_id": "load-u4", "username": "Load User 4", "is_active": true},
				{"user_id": "load-u5", "username": "Load User 5", "is_active": true},
			},
		}},

		{"Get Team 1", "GET", "/team/get?team_name=load-team-1", nil},
		{"Get Team 2", "GET", "/team/get?team_name=load-team-2", nil},

		{"Deactivate User", "POST", "/users/setIsActive", map[string]interface{}{
			"user_id": "load-u2", "is_active": false,
		}},

		{"Create PR 1", "POST", "/pullRequest/create", map[string]interface{}{
			"pull_request_id":   "load-pr-1",
			"pull_request_name": "Load Test PR 1",
			"author_id":         "load-u1",
		}},

		{"Create PR 2", "POST", "/pullRequest/create", map[string]interface{}{
			"pull_request_id":   "load-pr-2",
			"pull_request_name": "Load Test PR 2",
			"author_id":         "load-u4",
		}},

		{"Get User PRs", "GET", "/users/getReview?user_id=load-u3", nil},

		{"Merge PR 1", "POST", "/pullRequest/merge", map[string]interface{}{
			"pull_request_id": "load-pr-1",
		}},

		{"Health Check", "GET", "/health", nil},
	}

	client := &http.Client{Timeout: 10 * time.Second}

	// Делаем 5 проходов по всем тестам
	for round := 1; round <= 5; round++ {
		fmt.Printf("\n🎯 Round %d:\n", round)

		for _, test := range tests {
			var req *http.Request
			var err error

			if test.body != nil {
				jsonData, err := json.Marshal(test.body)
				if err != nil {
					log.Printf("❌ JSON error for %s: %v", test.name, err)
					continue
				}
				req, err = http.NewRequest(test.method, "http://localhost:8080"+test.path, bytes.NewBuffer(jsonData))
				if err != nil {
					log.Printf("❌ Request error for %s: %v", test.name, err)
					continue
				}
				req.Header.Set("Content-Type", "application/json")
			} else {
				req, err = http.NewRequest(test.method, "http://localhost:8080"+test.path, nil)
				if err != nil {
					log.Printf("❌ Request error for %s: %v", test.name, err)
					continue
				}
			}

			start := time.Now()
			resp, err := client.Do(req)
			duration := time.Since(start)

			if err != nil {
				fmt.Printf("❌ %s - Error: %v\n", test.name, err)
				continue
			}

			// Читаем ответ чтобы connection мог переиспользоваться
			buf := make([]byte, 1024)
			resp.Body.Read(buf)
			resp.Body.Close()

			emoji := "✅"
			if resp.StatusCode >= 400 {
				emoji = "⚠️"
			}

			fmt.Printf("%s %s - %d (%v)\n", emoji, test.name, resp.StatusCode, duration)

			// Небольшая пауза между запросами
			time.Sleep(100 * time.Millisecond)
		}

		// Пауза между раундами
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("\n🎉 Load test completed!")
	fmt.Println("📊 Check metrics at: http://localhost:8080/metrics/json")
	fmt.Println("📈 Check pretty dashboard at: http://localhost:8080/metrics-pretty")
}
