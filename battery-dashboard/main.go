package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jtfm/smartcharge/core/dynamodb"
	"github.com/rs/zerolog/log"
)

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

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get query parameters for time range (default to next 24 hours)
	hoursParam := request.QueryStringParameters["hours"]
	hours := 24
	if hoursParam != "" {
		if h, err := strconv.Atoi(hoursParam); err == nil && h > 0 && h <= 168 { // Max 1 week
			hours = h
		}
	}

	// Calculate time range
	now := time.Now()
	// Show what time zone the server is in
	log.Info().Msgf("Server time zone: %s", now.Location().String())
	end := now.Add(time.Duration(hours) * time.Hour)

	// Initialize DynamoDB client
	dbClient := dynamodb.InitDbClient(ctx)

	// Query system states
	systemStates, err := dbClient.ReadSystemStates(ctx, now, end)
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
		return handleJSONResponse(systemStates)
	}

	return handleHTMLResponse(systemStates)
}

func handleJSONResponse(systemStates []dynamodb.SystemState) (events.APIGatewayProxyResponse, error) {
	data := DashboardData{
		SystemStates: systemStates,
		Generated:    time.Now(),
	}

	jsonData, err := json.Marshal(data)
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
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Battery Dashboard - SmartCharge</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            margin: 0;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        h1 {
            color: #333;
            text-align: center;
            margin-bottom: 30px;
        }
        .chart-container {
            position: relative;
            height: 500px;
            margin: 20px 0;
        }
        .stats {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 20px;
            margin: 20px 0;
        }
        .stat-card {
            background: #f8f9fa;
            padding: 15px;
            border-radius: 6px;
            border-left: 4px solid #007bff;
        }
        .stat-value {
            font-size: 24px;
            font-weight: bold;
            color: #007bff;
        }
        .stat-label {
            color: #666;
            font-size: 14px;
        }
        .legend {
            margin: 20px 0;
            padding: 15px;
            background: #f8f9fa;
            border-radius: 6px;
        }
        .legend-item {
            display: inline-block;
            margin-right: 20px;
            margin-bottom: 5px;
        }
        .legend-color {
            display: inline-block;
            width: 20px;
            height: 15px;
            margin-right: 5px;
            vertical-align: middle;
        }
        .timestamp {
            text-align: center;
            color: #666;
            font-size: 12px;
            margin-top: 20px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔋 Battery Dashboard</h1>
        
        <div class="stats">
            <div class="stat-card">
                <div class="stat-value">{{.DataPoints}}</div>
                <div class="stat-label">Data Points</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">{{.ChargeSlots}}</div>
                <div class="stat-label">Planned Charge Slots</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">{{.MaxPrice}}p</div>
                <div class="stat-label">Max Unit Price</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">{{.MinPrice}}p</div>
                <div class="stat-label">Min Unit Price</div>
            </div>
        </div>

        <div class="legend">
            <div class="legend-item">
                <span class="legend-color" style="background-color: #36a2eb;"></span>
                Battery Power (kW)
            </div>
            <div class="legend-item">
                <span class="legend-color" style="background-color: #4bc0c0;"></span>
                Predicted Usage (kW)
            </div>
            <div class="legend-item">
                <span class="legend-color" style="background: linear-gradient(to right, #00c800, #ffa500, #ff0000); width: 100px;"></span>
                Unit Price (Green = Cheap, Red = Expensive)
            </div>
            <div class="legend-item">
                <span class="legend-color" style="background-color: rgba(100, 150, 255, 0.4);"></span>
                Charging Slots (Background)
            </div>
        </div>
        
        <div class="chart-container">
            <canvas id="batteryChart"></canvas>
        </div>

        <div class="timestamp">
            Generated: {{.Timestamp}}
        </div>
    </div>

    <script>
        const systemStates = {{.SystemStatesJSON}};
        
        // Prepare data for Chart.js
        const labels = systemStates.map(state => {
            const date = new Date(state.start_time);
            return date.toLocaleString('en-GB', { 
                month: 'short', 
                day: 'numeric', 
                hour: '2-digit', 
                minute: '2-digit'
            });
        });

        const batteryPowerData = systemStates.map(state => state.predicted_battery_power || 0);
        const predictedUsageData = systemStates.map(state => state.predicted_usage || 0);
        const unitPriceData = systemStates.map(state => state.unit_rate || 0);
        const chargeData = systemStates.map(state => state.will_charge ? (state.unit_rate || 0) : null);

        // Calculate min and max prices for color scaling
        const pricesForScaling = unitPriceData.filter(p => p > 0);
        const minPrice = Math.min(...pricesForScaling);
        const maxPrice = Math.max(...pricesForScaling);

        // Function to get color based on price value using continuous spectrum
        // Red (high) -> Orange -> Green (low)
        function getPriceColor(price) {
            if (price <= 0) return 'rgba(0, 0, 0, 0)'; // transparent for zero
            
            // Normalize price to 0-1 range where 0 = minPrice (green), 1 = maxPrice (red)
            const normalized = (price - minPrice) / (maxPrice - minPrice);
            
            // Create smooth spectrum: Red (1) -> Orange (0.5) -> Green (0)
            let r, g, b;
            
            if (normalized < 0.5) {
                // Green to Orange (0 to 0.5)
                const t = normalized * 2; // 0 to 1
                r = Math.round(255 * t); // 0 to 255
                g = 200; // constant high green
                b = 0;
            } else {
                // Orange to Red (0.5 to 1)
                const t = (normalized - 0.5) * 2; // 0 to 1
                r = 255; // constant high red
                g = Math.round(200 * (1 - t)); // 200 to 0
                b = 0;
            }
            
            return 'rgba(' + r + ', ' + g + ', ' + b + ', 0.8)';
        }

        // Create background colors for each price bar
        const priceBarColors = unitPriceData.map(price => getPriceColor(price));

        // Chart plugin to draw charging slot backgrounds
        const chargingBackgroundPlugin = {
            id: 'chargingBackground',
            afterDatasetsDraw(chart) {
                const ctx = chart.ctx;
                const xScale = chart.scales.x;
                const yScale = chart.scales.y;
                const chartArea = chart.chartArea;
                
                // Light electric blue for charging slots
                ctx.fillStyle = 'rgba(100, 150, 255, 0.15)';
                ctx.strokeStyle = 'rgba(100, 150, 255, 0.3)';
                ctx.lineWidth = 1;
                
                // Draw background rectangles for each charging slot
                systemStates.forEach((state, index) => {
                    if (state.will_charge) {
                        // Get pixel positions for this time slot
                        const xPixel = xScale.getPixelForValue(index);
                        const nextXPixel = index < systemStates.length - 1 ? 
                            xScale.getPixelForValue(index + 1) : 
                            xPixel + (xScale.width / systemStates.length);
                        
                        // Draw rectangle from bottom to top of chart
                        ctx.fillRect(
                            xPixel,
                            chartArea.top,
                            nextXPixel - xPixel,
                            chartArea.bottom - chartArea.top
                        );
                        ctx.strokeRect(
                            xPixel,
                            chartArea.top,
                            nextXPixel - xPixel,
                            chartArea.bottom - chartArea.top
                        );
                    }
                });
            }
        };

        const ctx = document.getElementById('batteryChart').getContext('2d');
        
        const chart = new Chart(ctx, {
            type: 'line',
            data: {
                labels: labels,
                datasets: [
                    {
                        label: 'Battery Power (kW)',
                        data: batteryPowerData,
                        borderColor: '#36a2eb',
                        backgroundColor: 'rgba(54, 162, 235, 0.1)',
                        borderWidth: 2,
                        fill: false,
                        yAxisID: 'y',
                        tension: 0.1
                    },
                    {
                        label: 'Predicted Usage (kW)',
                        data: predictedUsageData,
                        borderColor: '#4bc0c0',
                        backgroundColor: 'rgba(75, 192, 192, 0.1)',
                        borderWidth: 2,
                        fill: false,
                        yAxisID: 'y',
                        tension: 0.1
                    },
                    {
                        label: 'Unit Price (p/kWh)',
                        data: unitPriceData,
                        type: 'bar',
                        backgroundColor: priceBarColors,
                        borderColor: priceBarColors.map(c => c.replace('0.8', '1.0')),
                        borderWidth: 1,
                        yAxisID: 'y1'
                    }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                interaction: {
                    mode: 'index',
                    intersect: false,
                },
                plugins: {
                    title: {
                        display: true,
                        text: 'Battery State, Usage Predictions & Charging Schedule'
                    },
                    legend: {
                        display: false // We're using custom legend above
                    }
                },
                scales: {
                    x: {
                        display: true,
                        title: {
                            display: true,
                            text: 'Time'
                        },
                        ticks: {
                            maxTicksLimit: 12
                        }
                    },
                    y: {
                        type: 'linear',
                        display: true,
                        position: 'left',
                        title: {
                            display: true,
                            text: 'Power (kW)'
                        },
                        grid: {
                            drawOnChartArea: true,
                        },
                    },
                    y1: {
                        type: 'linear',
                        display: true,
                        position: 'right',
                        title: {
                            display: true,
                            text: 'Unit Price (p/kWh)'
                        },
                        grid: {
                            drawOnChartArea: false,
                        },
                    }
                }
            },
            plugins: [chargingBackgroundPlugin]
        });

        // Auto-refresh every 5 minutes
        setTimeout(() => {
            window.location.reload();
        }, 300000);
    </script>
</body>
</html>`

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

	t, err := template.New("dashboard").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var htmlBuffer strings.Builder
	err = t.Execute(&htmlBuffer, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return htmlBuffer.String(), nil
}
