package givenergy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

type EnergyUsageRequest struct {
	StartTime string       `json:"start_time"`
	EndTime   string       `json:"end_time"`
	Grouping  GroupingType `json:"grouping"`
	Types     []FlowType   `json:"types"`
}

type EnergyUsage struct {
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	EnergyUsage float32   `json:"energy_usage"`
}

type FlowType int

const (
	FlowType_PVToHome FlowType = iota
	FlowType_PVToBattery
	FlowType_PVToGrid
	FlowType_GridToHome
	FlowType_GridToBattery
	FlowType_BatteryToHome
	FlowType_BatteryToGrid
)

type GroupingType int

const (
	GroupingType_HalfHourly GroupingType = iota
	GroupingType_Daily
	GroupingType_Monthly
	GroupingType_Yearly
	GroupingType_Total
)

// Fetches energy usage data for the specified period
func (c *GivenergyClient) ReadUsage(start, end time.Time) ([]EnergyUsage, error) {
	const maxRecords = 1000                              // API limit
	const recordsPerDay = 48                             // Half-hourly intervals
	const maxDaysPerRequest = maxRecords / recordsPerDay // 20 days max per request
	grouping := GroupingType_HalfHourly

	var allUsage []EnergyUsage

	// Fetch data in chunks of maxDaysPerRequest
	for start.Before(end) || start.Equal(end) {
		// Determine chunk end date (max 20 days per request)
		chunkEnd := start.AddDate(0, 0, maxDaysPerRequest-1)
		if chunkEnd.After(end) {
			chunkEnd = end // Ensure we don’t exceed the requested range
		}

		// Fetch usage for this period
		log.Info().Msgf("Fetching usage data for period %s - %s", start, chunkEnd)
		usage, err := c.fetchUsageForPeriod(start, chunkEnd, grouping)
		if err != nil {
			return nil, err
		}

		// Append retrieved usage data
		allUsage = append(allUsage, usage...)

		// Move start date forward for next iteration
		start = chunkEnd.AddDate(0, 0, 1)
	}

	return allUsage, nil
}

func (c *GivenergyClient) fetchUsageForPeriod(start, end time.Time, grouping GroupingType) ([]EnergyUsage, error) {
	url := fmt.Sprintf("https://api.givenergy.cloud/v1/inverter/%s/energy-flows", c.InverterId)
	reqBody := EnergyUsageRequest{
		StartTime: start.Format("2006-01-02"),
		EndTime:   end.Format("2006-01-02"),
		Grouping:  grouping,
		Types:     []FlowType{FlowType_PVToHome, FlowType_GridToHome, FlowType_BatteryToHome},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	var rawResponse struct {
		Data map[string]struct {
			StartTime string             `json:"start_time"`
			EndTime   string             `json:"end_time"`
			Data      map[string]float32 `json:"data"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &rawResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response JSON: %v", err)
	}

	var processedResponse []EnergyUsage

	// Load Europe/London timezone for consistent behavior
	londonTz, err := time.LoadLocation("Europe/London")
	if err != nil {
		return nil, fmt.Errorf("failed to load Europe/London timezone: %v", err)
	}

	for _, entry := range rawResponse.Data {
		// Parse timestamps into time.Time - GivEnergy API returns times in UK local time
		layout := "2006-01-02 15:04"
		startTime, err := time.ParseInLocation(layout, entry.StartTime, londonTz)
		if err != nil {
			return nil, fmt.Errorf("failed to parse start_time: %v", err)
		}

		endTime, err := time.ParseInLocation(layout, entry.EndTime, londonTz)
		if err != nil {
			return nil, fmt.Errorf("failed to parse end_time: %v", err)
		}

		var totalUsage float32
		for _, value := range entry.Data {
			totalUsage += value
		}
		processedResponse = append(processedResponse, EnergyUsage{
			StartTime:   startTime,
			EndTime:     endTime,
			EnergyUsage: totalUsage,
		})
	}

	return processedResponse, nil
}
