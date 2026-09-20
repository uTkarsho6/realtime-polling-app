package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	baseURL      = "https://6bu1yb1kh3.execute-api.ap-south-1.amazonaws.com/Prod"
	totalVotes   = 1000
	concurrency  = 6 // Optimal for AWS account with 10 concurrent Lambda executions
)

type CreatePollRequest struct {
	Question string   `json:"question"`
	Options  []string `json:"options"`
}

type CreatePollResponse struct {
	PollId string `json:"pollId"`
}

type OptionCount struct {
	Count int `json:"count"`
}

type PollResponse struct {
	PollId   string                 `json:"pollId"`
	Question string                 `json:"question"`
	Options  map[string]OptionCount `json:"options"`
}

func main() {
	// Configure high-performance HTTP client with pooled connections
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}

	fmt.Println("================================================================")
	fmt.Println("🚀 PulsePoll Real-Time Serverless Load Test — 1,000 Votes")
	fmt.Println("================================================================")
	fmt.Printf("Target Endpoint: %s\n", baseURL)
	fmt.Printf("Total Votes:     %d\n", totalVotes)
	fmt.Printf("Worker Concurrency: %d\n\n", concurrency)

	// Step 1: Create a fresh test poll
	fmt.Println("Step 1: Creating fresh test poll...")
	pollReq := CreatePollRequest{
		Question: "1,000 Concurrent Votes Concurrency Stress Test",
		Options:  []string{"Option_A", "Option_B", "Option_C"},
	}
	reqBytes, _ := json.Marshal(pollReq)
	resp, err := client.Post(baseURL+"/polls", "application/json", bytes.NewReader(reqBytes))
	if err != nil {
		panic(fmt.Sprintf("Failed to create poll: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		panic(fmt.Sprintf("Create poll failed with HTTP %d: %s", resp.StatusCode, string(body)))
	}

	var createResp CreatePollResponse
	json.NewDecoder(resp.Body).Decode(&createResp)
	pollId := createResp.PollId
	fmt.Printf("✅ Created test poll with ID: %s\n\n", pollId)

	// Step 2: Prepare vote distribution
	// 500 for Option_A, 300 for Option_B, 200 for Option_C
	expectedA := 500
	expectedB := 300
	expectedC := 200

	votesQueue := make([]string, 0, totalVotes)
	for i := 0; i < expectedA; i++ {
		votesQueue = append(votesQueue, "Option_A")
	}
	for i := 0; i < expectedB; i++ {
		votesQueue = append(votesQueue, "Option_B")
	}
	for i := 0; i < expectedC; i++ {
		votesQueue = append(votesQueue, "Option_C")
	}

	// Step 3: Run Concurrent Load Test
	fmt.Println("Step 2: Firing 1,000 concurrent votes...")
	voteChan := make(chan string, totalVotes)
	for _, v := range votesQueue {
		voteChan <- v
	}
	close(voteChan)

	var (
		wg          sync.WaitGroup
		successA    int64
		successB    int64
		successC    int64
		failedVotes int64
		latencies   []time.Duration
		latMu       sync.Mutex
	)

	startTime := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for option := range voteChan {
				payload, _ := json.Marshal(map[string]string{"option": option})
				voteURL := fmt.Sprintf("%s/polls/%s/vote", baseURL, pollId)

				var (
					vResp    *http.Response
					err      error
					duration time.Duration
					success  bool
				)

				// Retry up to 3 times on transient Lambda concurrency limit throttling
				for attempt := 0; attempt < 4; attempt++ {
					t0 := time.Now()
					vResp, err = client.Post(voteURL, "application/json", bytes.NewReader(payload))
					duration = time.Since(t0)

					if err == nil && vResp.StatusCode == http.StatusOK {
						success = true
						vResp.Body.Close()
						break
					}

					if vResp != nil {
						vResp.Body.Close()
					}
					// Exponential backoff: 25ms, 50ms, 100ms
					time.Sleep(time.Duration(25*(1<<attempt)) * time.Millisecond)
				}

				latMu.Lock()
				latencies = append(latencies, duration)
				latMu.Unlock()

				if !success {
					atomic.AddInt64(&failedVotes, 1)
					continue
				}

				switch option {
				case "Option_A":
					atomic.AddInt64(&successA, 1)
				case "Option_B":
					atomic.AddInt64(&successB, 1)
				case "Option_C":
					atomic.AddInt64(&successC, 1)
				}
			}
		}()
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	// Step 4: Calculate Metrics
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	var totalLatency time.Duration
	for _, l := range latencies {
		totalLatency += l
	}
	avgLatency := totalLatency / time.Duration(len(latencies))
	p50 := latencies[len(latencies)*50/100]
	p95 := latencies[len(latencies)*95/100]
	p99 := latencies[len(latencies)*99/100]
	rps := float64(totalVotes) / totalDuration.Seconds()

	fmt.Println("\n================================================================")
	fmt.Println("📊 Load Test Performance Results")
	fmt.Println("================================================================")
	fmt.Printf("Total Requests Sent: %d\n", totalVotes)
	fmt.Printf("Total Duration:      %v\n", totalDuration)
	fmt.Printf("Throughput (RPS):    %.2f requests/sec\n", rps)
	fmt.Printf("Successful Requests: %d\n", successA+successB+successC)
	fmt.Printf("Failed Requests:     %d\n", failedVotes)
	fmt.Println("----------------------------------------------------------------")
	fmt.Printf("Latency Avg:         %v\n", avgLatency)
	fmt.Printf("Latency p50:         %v\n", p50)
	fmt.Printf("Latency p95:         %v\n", p95)
	fmt.Printf("Latency p99:         %v\n", p99)
	fmt.Println("================================================================")

	// Step 5: Verify DynamoDB Consistency (Zero Lost Updates)
	fmt.Println("\nStep 3: Verifying DynamoDB Data Consistency...")
	getResp, err := client.Get(fmt.Sprintf("%s/polls/%s", baseURL, pollId))
	if err != nil {
		panic(fmt.Sprintf("Failed to fetch poll from DynamoDB: %v", err))
	}
	defer getResp.Body.Close()

	var finalPoll PollResponse
	json.NewDecoder(getResp.Body).Decode(&finalPoll)

	countA := finalPoll.Options["Option_A"].Count
	countB := finalPoll.Options["Option_B"].Count
	countC := finalPoll.Options["Option_C"].Count
	totalCount := countA + countB + countC

	fmt.Printf("Option_A: %d / %d expected\n", countA, expectedA)
	fmt.Printf("Option_B: %d / %d expected\n", countB, expectedB)
	fmt.Printf("Option_C: %d / %d expected\n", countC, expectedC)
	fmt.Printf("Total Counted in DynamoDB: %d / %d\n", totalCount, totalVotes)

	if countA == expectedA && countB == expectedB && countC == expectedC && totalCount == totalVotes {
		fmt.Println("\n🎉 PASS! ZERO LOST UPDATES: 1,000 / 1,000 votes accurately recorded under high concurrency!")
	} else {
		fmt.Printf("\n❌ FAIL! Data inconsistency detected. Missing %d votes.\n", totalVotes-totalCount)
	}
	fmt.Println("================================================================")
}
