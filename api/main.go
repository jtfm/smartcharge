package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jtfm/smartcharge/core/dynamodb"
	"github.com/jtfm/smartcharge/core/givenergy"
)

var ddbClient = dynamodb.InitDbClient(context.Background())

type EnergyUsageResponse struct {
	StartTime     string  `json:"start_time"`
	EndTime       string  `json:"end_time"`
	EnergyUsage   float32 `json:"energy_usage"`
	FormattedDate string  `json:"formatted_date"`
	FormattedTime string  `json:"formatted_time"`
}

type ApiResponse struct {
	Data  []EnergyUsageResponse `json:"data"`
	Total int                   `json:"total"`
	Stats Stats                 `json:"stats"`
}

type Stats struct {
	TotalUsage float32 `json:"total_usage"`
	AvgUsage   float32 `json:"avg_usage"`
	MaxUsage   float32 `json:"max_usage"`
	MinUsage   float32 `json:"min_usage"`
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Enable CORS
	headers := map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Headers": "Content-Type",
		"Access-Control-Allow-Methods": "GET, POST, OPTIONS",
		"Content-Type":                 "application/json",
	}

	// Handle preflight OPTIONS request
	if request.HTTPMethod == "OPTIONS" {
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Headers:    headers,
		}, nil
	}

	// Parse query parameters for date range
	var startTime, endTime time.Time
	var err error

	// Default to last 7 days if no parameters provided
	if startParam := request.QueryStringParameters["start"]; startParam != "" {
		if startUnix, err := strconv.ParseInt(startParam, 10, 64); err == nil {
			startTime = time.Unix(startUnix, 0)
		} else {
			startTime, err = time.Parse("2006-01-02", startParam)
			if err != nil {
				return events.APIGatewayProxyResponse{
					StatusCode: 400,
					Headers:    headers,
					Body:       `{"error": "Invalid start date format. Use YYYY-MM-DD or Unix timestamp"}`,
				}, nil
			}
		}
	} else {
		startTime = time.Now().AddDate(0, 0, -7) // Last 7 days
	}

	if endParam := request.QueryStringParameters["end"]; endParam != "" {
		if endUnix, err := strconv.ParseInt(endParam, 10, 64); err == nil {
			endTime = time.Unix(endUnix, 0)
		} else {
			endTime, err = time.Parse("2006-01-02", endParam)
			if err != nil {
				return events.APIGatewayProxyResponse{
					StatusCode: 400,
					Headers:    headers,
					Body:       `{"error": "Invalid end date format. Use YYYY-MM-DD or Unix timestamp"}`,
				}, nil
			}
		}
	} else {
		endTime = time.Now()
	}

	// Retrieve energy usage data
	energyUsages, err := ddbClient.ReadEnergyUsages(ctx, startTime, endTime)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Headers:    headers,
			Body:       fmt.Sprintf(`{"error": "Failed to retrieve energy usage data: %v"}`, err),
		}, nil
	}

	// Transform data for frontend
	response := transformEnergyUsageData(energyUsages)

	// Marshal response to JSON
	responseBody, err := json.Marshal(response)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Headers:    headers,
			Body:       `{"error": "Failed to marshal response"}`,
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(responseBody),
	}, nil
}

func transformEnergyUsageData(energyUsages []givenergy.EnergyUsage) ApiResponse {
	data := make([]EnergyUsageResponse, len(energyUsages))
	var totalUsage, maxUsage, minUsage float32
	minUsage = 999999 // Initialize to high value

	for i, usage := range energyUsages {
		data[i] = EnergyUsageResponse{
			StartTime:     fmt.Sprintf("%d", usage.StartTime.Unix()),
			EndTime:       fmt.Sprintf("%d", usage.EndTime.Unix()),
			EnergyUsage:   usage.EnergyUsage,
			FormattedDate: usage.StartTime.Format("2006-01-02"),
			FormattedTime: usage.StartTime.Format("15:04"),
		}

		// Calculate stats
		totalUsage += usage.EnergyUsage
		if usage.EnergyUsage > maxUsage {
			maxUsage = usage.EnergyUsage
		}
		if usage.EnergyUsage < minUsage {
			minUsage = usage.EnergyUsage
		}
	}

	var avgUsage float32
	if len(energyUsages) > 0 {
		avgUsage = totalUsage / float32(len(energyUsages))
	}

	// Reset minUsage if no data
	if len(energyUsages) == 0 {
		minUsage = 0
	}

	return ApiResponse{
		Data:  data,
		Total: len(energyUsages),
		Stats: Stats{
			TotalUsage: totalUsage,
			AvgUsage:   avgUsage,
			MaxUsage:   maxUsage,
			MinUsage:   minUsage,
		},
	}
}

func main() {
	lambda.Start(handler)
}
