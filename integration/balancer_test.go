package integration

import (
	"net/http"
	"os"
	"testing"
	"time"
)

const (
	balancerHost    = "http://balancer:8090"
	apiEndpoint     = "/api/v1/some-data"
	serverHeader    = "lb-from"
	minServers      = 2
	requestTimeout  = 3 * time.Second
	requestInterval = 100 * time.Millisecond
)

type loadBalancerTester struct {
	client      *http.Client
	requestURL  string
	serverStats map[string]int
}

func newLoadBalancerTester() *loadBalancerTester {
	return &loadBalancerTester{
		client: &http.Client{
			Timeout: requestTimeout,
		},
		requestURL:  balancerHost + apiEndpoint,
		serverStats: make(map[string]int),
	}
}

func (t *loadBalancerTester) makeRequest() (string, error) {
	resp, err := t.client.Get(t.requestURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	serverID := resp.Header.Get(serverHeader)
	if serverID == "" {
		return "", nil
	}

	t.serverStats[serverID]++
	return serverID, nil
}

func (t *loadBalancerTester) verifyDistribution(minExpected int) bool {
	return len(t.serverStats) >= minExpected
}

func TestRequestDistribution(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test (set INTEGRATION_TEST to run)")
	}

	tester := newLoadBalancerTester()
	const requestCount = 10

	for i := 0; i < requestCount; i++ {
		serverID, err := tester.makeRequest()
		if err != nil {
			t.Fatalf("Request %d failed: %v", i+1, err)
		}
		if serverID == "" {
			t.Fatalf("Request %d: missing server identifier", i+1)
		}
		t.Logf("Request %d handled by: %s", i+1, serverID)
		time.Sleep(requestInterval)
	}

	if !tester.verifyDistribution(minServers) {
		t.Errorf("Expected requests to be distributed to at least %d servers, got %d: %v",
			minServers, len(tester.serverStats), tester.serverStats)
	} else {
		t.Logf("Successful distribution across %d servers: %v",
			len(tester.serverStats), tester.serverStats)
	}
}

func TestBalancerPerformance(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping performance test (set INTEGRATION_TEST to run)")
	}

	tester := newLoadBalancerTester()
	const benchmarkRequests = 20

	start := time.Now()
	for i := 0; i < benchmarkRequests; i++ {
		if _, err := tester.makeRequest(); err != nil {
			t.Fatalf("Performance request %d failed: %v", i+1, err)
		}
	}
	duration := time.Since(start)

	t.Logf("Completed %d requests in %v (avg %v/request)",
		benchmarkRequests, duration, duration/time.Duration(benchmarkRequests))
}
