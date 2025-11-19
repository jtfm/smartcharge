package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jtfm/smartcharge/core/dynamodb"
	"github.com/rs/zerolog/log"
)

//go:embed templates/*
var templatesFS embed.FS

type DashboardData struct {
	SystemStates []dynamodb.SystemState `json:"system_states"`
	Generated    time.Time              `json:"generated"`
}

func main() {
	if isInLambda() {
		lambda.Start(handler)
	} else {
		// Local development - start HTTP server
		port := ":8080"

		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Convert HTTP request to API Gateway format
			queryParams := make(map[string]string)
			for key, values := range r.URL.Query() {
				if len(values) > 0 {
					queryParams[key] = values[0]
				}
			}

			request := events.APIGatewayProxyRequest{
				QueryStringParameters: queryParams,
				Headers:               convertHeaders(r.Header),
			}

			response, err := handler(r.Context(), request)
			if err != nil {
				log.Error().Err(err).Msg("Error calling handler")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Internal server error"))
				return
			}

			// Write response
			for key, value := range response.Headers {
				w.Header().Set(key, value)
			}
			w.WriteHeader(response.StatusCode)
			w.Write([]byte(response.Body))
		})

		log.Info().Msgf("Starting local server on http://localhost%s", port)
		if err := http.ListenAndServe(port, nil); err != nil {
			log.Fatal().Err(err).Msg("Server failed to start")
		}
	}
}

// convertHeaders converts http.Header to map[string]string for API Gateway format
func convertHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)
	for key, values := range headers {
		if len(values) > 0 {
			result[key] = values[0]
		}
	}
	return result
}

func isInLambda() bool {
	return os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != ""
}

func calculateHoursRange(now time.Time, hoursParam string) (time.Time, time.Time) {
	hours := 24
	if hoursParam != "" {
		if h, err := strconv.Atoi(hoursParam); err == nil && h > 0 && h <= 730 { // Max 30 days
			hours = h
		}
	}

	end := now.Add(time.Duration(hours) * time.Hour)
	return now, end
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	now := time.Now()
	// Show what time zone the server is in
	log.Info().Msgf("Server time zone: %s", now.Location().String())

	// Check for explicit date range parameters first
	fromDateParam := request.QueryStringParameters["fromDate"]
	toDateParam := request.QueryStringParameters["toDate"]

	var start, end time.Time

	// If date range is provided, use it
	if fromDateParam != "" && toDateParam != "" {
		// Try parsing as ISO 8601 date format (YYYY-MM-DD)
		parsedFrom, errFrom := time.Parse("2006-01-02", fromDateParam)
		parsedTo, errTo := time.Parse("2006-01-02", toDateParam)

		if errFrom == nil && errTo == nil {
			start = parsedFrom
			// Set end to end of day
			end = parsedTo.Add(24*time.Hour - time.Second)
			log.Info().Msgf("Using date range: %s to %s", start.Format("2006-01-02"), end.Format("2006-01-02"))
		} else {
			// Fall back to hours parameter if date parsing fails
			start, end = calculateHoursRange(now, request.QueryStringParameters["hours"])
		}
	} else {
		// Use hours parameter (default to next 24 hours)
		start, end = calculateHoursRange(now, request.QueryStringParameters["hours"])
	}

	// Initialize DynamoDB client
	dbClient := dynamodb.InitDbClient(ctx)

	// Query system states
	systemStates, err := dbClient.ReadSystemStates(ctx, start, end)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read system states")
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "Internal server error",
		}, nil
	}

	// Handle different response types based on Accept header or format parameter
	format := request.QueryStringParameters["format"]
	acceptHeader := request.Headers["Accept"]

	if format == "json" || acceptHeader == "application/json" {
		return handleJSONResponse(systemStates, request.QueryStringParameters)
	}

	return handleHTMLResponse(systemStates)
}

func handleJSONResponse(systemStates []dynamodb.SystemState, queryParams map[string]string) (events.APIGatewayProxyResponse, error) {
	// Parse pagination parameters
	page := 1
	pageSize := 100

	if p, err := strconv.Atoi(queryParams["page"]); err == nil && p > 0 {
		page = p
	}
	if ps, err := strconv.Atoi(queryParams["pageSize"]); err == nil && ps > 0 && ps <= 1000 {
		pageSize = ps
	}

	// Parse sorting parameters
	sortBy := queryParams["sortBy"]
	sortOrder := queryParams["sortOrder"] // "asc" or "desc"
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "asc"
	}

	// Apply sorting
	systemStates = sortSystemStates(systemStates, sortBy, sortOrder)

	// Apply pagination
	totalRecords := len(systemStates)
	start := (page - 1) * pageSize
	end := start + pageSize

	if start >= totalRecords {
		start = totalRecords
	}
	if end > totalRecords {
		end = totalRecords
	}

	paginatedStates := systemStates[start:end]

	// Create response with pagination metadata
	responseData := map[string]interface{}{
		"system_states": paginatedStates,
		"generated":     time.Now(),
		"pagination": map[string]interface{}{
			"page":         page,
			"pageSize":     pageSize,
			"totalRecords": totalRecords,
			"totalPages":   (totalRecords + pageSize - 1) / pageSize,
			"hasNextPage":  end < totalRecords,
			"hasPrevPage":  start > 0,
		},
		"sorting": map[string]interface{}{
			"sortBy":    sortBy,
			"sortOrder": sortOrder,
		},
	}

	jsonData, err := json.Marshal(responseData)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "Failed to marshal JSON",
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type":                 "application/json",
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "GET, OPTIONS",
			"Access-Control-Allow-Headers": "Content-Type",
		},
		Body: string(jsonData),
	}, nil
}

func sortSystemStates(states []dynamodb.SystemState, sortBy, sortOrder string) []dynamodb.SystemState {
	// Create a copy to avoid modifying the original
	sorted := make([]dynamodb.SystemState, len(states))
	copy(sorted, states)

	// Define comparator based on sortBy parameter
	switch sortBy {
	case "start_time":
		if sortOrder == "desc" {
			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i].StartTime.After(sorted[j].StartTime)
			})
		} else {
			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i].StartTime.Before(sorted[j].StartTime)
			})
		}
	case "pv_estimate":
		if sortOrder == "desc" {
			sort.Slice(sorted, func(i, j int) bool {
				return floatPtrValue(sorted[i].PvEstimate) > floatPtrValue(sorted[j].PvEstimate)
			})
		} else {
			sort.Slice(sorted, func(i, j int) bool {
				return floatPtrValue(sorted[i].PvEstimate) < floatPtrValue(sorted[j].PvEstimate)
			})
		}
	case "predicted_usage":
		if sortOrder == "desc" {
			sort.Slice(sorted, func(i, j int) bool {
				return floatPtrValue(sorted[i].PredictedUsage) > floatPtrValue(sorted[j].PredictedUsage)
			})
		} else {
			sort.Slice(sorted, func(i, j int) bool {
				return floatPtrValue(sorted[i].PredictedUsage) < floatPtrValue(sorted[j].PredictedUsage)
			})
		}
	case "predicted_battery_power":
		if sortOrder == "desc" {
			sort.Slice(sorted, func(i, j int) bool {
				return floatPtrValue(sorted[i].PredictedBatteryPower) > floatPtrValue(sorted[j].PredictedBatteryPower)
			})
		} else {
			sort.Slice(sorted, func(i, j int) bool {
				return floatPtrValue(sorted[i].PredictedBatteryPower) < floatPtrValue(sorted[j].PredictedBatteryPower)
			})
		}
	case "unit_rate":
		if sortOrder == "desc" {
			sort.Slice(sorted, func(i, j int) bool {
				return floatPtrValue(sorted[i].UnitRate) > floatPtrValue(sorted[j].UnitRate)
			})
		} else {
			sort.Slice(sorted, func(i, j int) bool {
				return floatPtrValue(sorted[i].UnitRate) < floatPtrValue(sorted[j].UnitRate)
			})
		}
	case "will_charge":
		if sortOrder == "desc" {
			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i].WillCharge && !sorted[j].WillCharge
			})
		} else {
			sort.Slice(sorted, func(i, j int) bool {
				return !sorted[i].WillCharge && sorted[j].WillCharge
			})
		}
	default:
		// Default sort by start_time ascending
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].StartTime.Before(sorted[j].StartTime)
		})
	}

	return sorted
}

func floatPtrValue(ptr *float64) float64 {
	if ptr == nil {
		return 0
	}
	return *ptr
}

func handleHTMLResponse(systemStates []dynamodb.SystemState) (events.APIGatewayProxyResponse, error) {
	htmlContent, err := generateHTML(systemStates)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate HTML")
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "Failed to generate HTML",
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type":                 "text/html",
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "GET, OPTIONS",
			"Access-Control-Allow-Headers": "Content-Type",
		},
		Body: htmlContent,
	}, nil
}

func generateHTML(systemStates []dynamodb.SystemState) (string, error) {
	// Calculate statistics
	dataPoints := len(systemStates)
	chargeSlots := 0
	maxPrice := 0.0
	minPrice := 999.0

	for _, state := range systemStates {
		if state.WillCharge {
			chargeSlots++
		}
		if state.UnitRate != nil {
			if *state.UnitRate > maxPrice {
				maxPrice = *state.UnitRate
			}
			if *state.UnitRate < minPrice {
				minPrice = *state.UnitRate
			}
		}
	}

	if len(systemStates) == 0 {
		minPrice = 0
	}

	// Convert system states to JSON for JavaScript
	systemStatesJSON, err := json.Marshal(systemStates)
	if err != nil {
		return "", fmt.Errorf("failed to marshal system states: %w", err)
	}

	// Template data
	data := struct {
		SystemStatesJSON template.JS
		DataPoints       int
		ChargeSlots      int
		MaxPrice         string
		MinPrice         string
		Timestamp        string
	}{
		SystemStatesJSON: template.JS(systemStatesJSON),
		DataPoints:       dataPoints,
		ChargeSlots:      chargeSlots,
		MaxPrice:         fmt.Sprintf("%.1f", maxPrice),
		MinPrice:         fmt.Sprintf("%.1f", minPrice),
		Timestamp:        time.Now().Format("2006-01-02 15:04:05 MST"),
	}

	// Parse all templates from embedded filesystem
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return "", fmt.Errorf("failed to parse templates: %w", err)
	}

	var htmlBuffer strings.Builder
	err = tmpl.ExecuteTemplate(&htmlBuffer, "dashboard", data)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return htmlBuffer.String(), nil
}
