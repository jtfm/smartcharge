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
	<script src="https://cdn.plot.ly/plotly-2.27.0.min.js"></script>
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
				<span class="legend-color" style="background-color: #0000FF;"></span>
				Battery Power (kW)
			</div>
			<div class="legend-item">
				<span class="legend-color" style="background-color: #FFD700;"></span>
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
			<div id="batteryChart"></div>
		</div>

		<div class="timestamp">
			Generated: {{.Timestamp}}
		</div>
	</div>

	<script>
		const systemStates = {{.SystemStatesJSON}};
		
		// Prepare time arrays - use actual timestamps for proper time-based positioning
		const times = systemStates.map(state => state.start_time);
		const batteryPowerData = systemStates.map(state => state.predicted_battery_power || 0);
		const predictedUsageData = systemStates.map(state => state.predicted_usage || 0);
		const unitPriceData = systemStates.map(state => state.unit_rate || 0);

		// For bars: shift x values by 15 minutes (half of 30 min slot) so bars start at slot beginning
		// Calculate midpoint times for bars (15 minutes = 900000 ms after start)
		const barTimes = systemStates.map(state => {
			const startTime = new Date(state.start_time);
			return new Date(startTime.getTime() + 900000).toISOString(); // +15 minutes
		});

		// Calculate min and max prices for color scaling
		const pricesForScaling = unitPriceData.filter(p => p > 0);
		const minPrice = Math.min(...pricesForScaling);
		const maxPrice = Math.max(...pricesForScaling);

		// Function to get color based on price value using continuous spectrum
		function getPriceColor(price) {
			if (price <= 0) return 'rgba(0, 0, 0, 0)';
			
			const normalized = (price - minPrice) / (maxPrice - minPrice);
			let r, g, b;
			
			if (normalized < 0.5) {
				const t = normalized * 2;
				r = Math.round(255 * t);
				g = 200;
				b = 0;
			} else {
				const t = (normalized - 0.5) * 2;
				r = 255;
				g = Math.round(200 * (1 - t));
				b = 0;
			}
			
			return 'rgba(' + r + ', ' + g + ', ' + b + ', 0.4)';
		}

		// Create colors for each price bar
		const priceBarColors = unitPriceData.map(price => getPriceColor(price));

		// Create shapes for charging slot backgrounds and 30-minute interval grid
		const shapes = [];
		
		// Add 30-minute interval grid lines
		systemStates.forEach((state, index) => {
			if (index < systemStates.length - 1) {
				shapes.push({
					type: 'line',
					xref: 'x',
					yref: 'paper',
					x0: state.start_time,
					x1: state.start_time,
					y0: 0,
					y1: 1,
					line: {
						color: 'rgba(200, 200, 200, 0.3)',
						width: 1,
						dash: 'dot'
					},
					layer: 'below'
				});
			}
		});

		// Add charging slot backgrounds
		systemStates.forEach((state, index) => {
			if (state.will_charge && index < systemStates.length - 1) {
				shapes.push({
					type: 'rect',
					xref: 'x',
					yref: 'paper',
					x0: state.start_time,
					x1: systemStates[index + 1].start_time,
					y0: 0,
					y1: 1,
					fillcolor: 'rgba(100, 150, 255, 0.15)',
					line: {
						width: 0
					},
					layer: 'below'
				});
			}
		});

		// Create traces - price bars first, then lines on top
		const traces = [
			{
				name: 'Unit Price (p/kWh)',
				x: barTimes,
				y: unitPriceData,
				type: 'bar',
				marker: {
					color: priceBarColors,
					line: {
						width: 1,
						color: priceBarColors.map(c => c.replace('0.8', '1.0'))
					}
				},
				yaxis: 'y2',
				width: 1800000, // 30 minutes in milliseconds
				hoverlabel: {
					bgcolor: 'rgba(255, 255, 255, 0.9)',
					bordercolor: '#333'
				}
			},
			{
				name: 'Battery Power (kW)',
				x: times,
				y: batteryPowerData,
				type: 'scatter',
				mode: 'lines',
				line: {
					color: '#0000FF',
					width: 3
				},
				yaxis: 'y'
			},
			{
				name: 'Predicted Usage (kW)',
				x: times,
				y: predictedUsageData,
				type: 'scatter',
				mode: 'lines',
				line: {
					color: '#FFFF00',
					width: 3
				},
				yaxis: 'y'
			}
		];

		// Layout configuration
		const layout = {
			title: 'Battery State, Usage Predictions & Charging Schedule',
			showlegend: false,
			xaxis: {
				title: 'Time',
				type: 'date',
				tickformat: '%b %d\n%H:%M',
				gridcolor: 'rgba(0, 0, 0, 0.1)',
				dtick: 7200000, // 2 hours in milliseconds (reduced from 30 minutes)
				tick0: systemStates.length > 0 ? systemStates[0].start_time : null
			},
			yaxis: {
				title: 'Power (kW)',
				side: 'left',
				gridcolor: 'rgba(0, 0, 0, 0.1)'
			},
			yaxis2: {
				title: 'Unit Price (p/kWh)',
				side: 'right',
				overlaying: 'y',
				rangemode: 'tozero',
				showgrid: false
			},
			shapes: shapes,
			hovermode: 'x unified',
			hoverdistance: 50,
			margin: {
				l: 60,
				r: 60,
				t: 80,
				b: 60
			}
		};

		// Configuration
		const config = {
			responsive: true,
			displayModeBar: true,
			displaylogo: false,
			modeBarButtonsToRemove: ['lasso2d', 'select2d']
		};

		// Create the plot
		Plotly.newPlot('batteryChart', traces, layout, config);

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
