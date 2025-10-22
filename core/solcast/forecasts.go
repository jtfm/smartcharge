package solcast

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func UnmarshalForecasts(data []byte) (*Forecasts, error) {
	var r Forecasts
	err := json.Unmarshal(data, &r)
	return &r, err
}

func (r *Forecasts) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type Forecasts struct {
	Forecasts []Forecast `json:"forecasts"`
}

type Forecast struct {
	PeriodEnd    time.Time `json:"period_end"`
	Period       string    `json:"period"` // Length of the period in ISO 8601 format
	PvEstimate   float64   `json:"pv_estimate"`
	PvEstimate10 float64   `json:"pv_estimate10"`
	PvEstimate90 float64   `json:"pv_estimate90"`
}

// InternalForecast is the internal representation of a Forecast that is stored in DynamoDB
// The StartTime is calculated from the PeriodEnd and Period fields
type InternalForecast struct {
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	PvEstimate   float64   `json:"pv_estimate"`
	PvEstimate10 float64   `json:"pv_estimate_10"`
	PvEstimate90 float64   `json:"pv_estimate_90"`
}

// Convert a Forecast to InternalForecast
func ToInternal(f Forecast) (*InternalForecast, error) {
	// Load Europe/London timezone for consistent behavior
	londonTz, err := time.LoadLocation("Europe/London")
	if err != nil {
		return &InternalForecast{}, fmt.Errorf("failed to load Europe/London timezone: %v", err)
	}

	// Parse the period duration from the ISO 8601 format
	periodDuration, err := time.ParseDuration(strings.ToLower(strings.TrimPrefix(f.Period, "PT")))
	if err != nil {
		return &InternalForecast{}, fmt.Errorf("invalid period format: %v", err)
	}

	// Calculate StartTime from PeriodEnd
	startTime := f.PeriodEnd.Add(-periodDuration)

	// Convert UTC times to Europe/London timezone
	// Solcast API returns timestamps in UTC, but we need UK local time to match system states
	localEndTime := f.PeriodEnd.In(londonTz)
	localStartTime := startTime.In(londonTz)

	// Log timezone conversion for debugging (first few forecasts only)
	if f.PvEstimate > 0 { // Only log when there's actual solar generation
		fmt.Printf("Solcast timezone conversion: UTC %s -> Europe/London %s\n",
			startTime.Format("2006-01-02 15:04:05 MST"),
			localStartTime.Format("2006-01-02 15:04:05 MST"))
	}

	// Create the InternalForecast with the converted fields
	internal := InternalForecast{
		StartTime:    localStartTime,
		EndTime:      localEndTime,
		PvEstimate:   f.PvEstimate,
		PvEstimate10: f.PvEstimate10,
		PvEstimate90: f.PvEstimate90,
	}

	return &internal, nil
}
