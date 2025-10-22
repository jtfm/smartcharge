package main
package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/jtfm/smartcharge/core/dynamodb"
)

func TestHandler(t *testing.T) {
	// Mock data for testing
	ctx := context.Background()
	
	// Test JSON response
	request := events.APIGatewayProxyRequest{
		QueryStringParameters: map[string]string{
			"format": "json",
			"hours":  "1",
		},
	}

	response, err := handler(ctx, request)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if response.StatusCode != 200 {
		t.Errorf("expected status code 200, got %d", response.StatusCode)
	}

	// Test that response is valid JSON
	var data DashboardData
	err = json.Unmarshal([]byte(response.Body), &data)
	if err != nil {
		t.Errorf("response body is not valid JSON: %v", err)
	}
}

func TestGenerateHTML(t *testing.T) {
	// Test HTML generation with mock data
	mockStates := []dynamodb.SystemState{
		{
			StartTime:             time.Now(),
			PvEstimate:            float64Ptr(2.5),
			PredictedUsage:        float64Ptr(1.8),
			PredictedBatteryPower: float64Ptr(0.7),
			UnitRate:              float64Ptr(15.2),
			WillCharge:            false,
		},
		{
			StartTime:             time.Now().Add(30 * time.Minute),
			PvEstimate:            float64Ptr(3.0),
			PredictedUsage:        float64Ptr(2.0),
			PredictedBatteryPower: float64Ptr(1.0),
			UnitRate:              float64Ptr(12.8),
			WillCharge:            true,
		},
	}

	html, err := generateHTML(mockStates)
	if err != nil {
		t.Fatalf("generateHTML returned error: %v", err)
	}

	if len(html) == 0 {
		t.Error("generated HTML is empty")
	}

	// Check for expected content
	expectedStrings := []string{
		"Battery Dashboard",
		"Chart.js",
		"Battery Power (kW)",
		"Unit Price (p/kWh)",
		"Charging Slots",
	}

	for _, expected := range expectedStrings {
		if !contains(html, expected) {
			t.Errorf("generated HTML does not contain expected string: %s", expected)
		}
	}
}

func float64Ptr(f float64) *float64 {
	return &f
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || 
		s[len(s)-len(substr):] == substr || 
		containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}