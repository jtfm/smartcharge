package givenergy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/rs/zerolog/log"
)

// Define the Setting type as an integer
type Setting int

// Define constants for each setting ID using iota
const (
	Setting_EnableACChargeUpperLimit  Setting = 17
	Setting_EnableEcoMode             Setting = 24
	Setting_DcDischarge2StartTime     Setting = 41
	Setting_DcDischarge2EndTime       Setting = 42
	Setting_InverterMaxOutputPower    Setting = 47
	Setting_DcDischarge1StartTime     Setting = 53
	Setting_DcDischarge1EndTime       Setting = 54
	Setting_EnableDCDischarge         Setting = 56
	Setting_ACCharge1StartTime        Setting = 64
	Setting_ACCharge1EndTime          Setting = 65
	Setting_ACChargeEnable            Setting = 66
	Setting_BatteryReserveLimit       Setting = 71
	Setting_BatteryChargePower        Setting = 72
	Setting_BatteryDischargePower     Setting = 73
	Setting_BatteryCutoffLimit        Setting = 75
	Setting_ACChargeUpperLimit        Setting = 77
	Setting_RestartInverter           Setting = 83
	Setting_PauseBattery              Setting = 96
	Setting_PauseBatteryStartTime     Setting = 155
	Setting_PauseBatteryEndTime       Setting = 156
	Setting_ExportPowerPriority       Setting = 265
	Setting_InverterChargePowerPct    Setting = 267
	Setting_InverterDischargePowerPct Setting = 268
	Setting_EnableEPS                 Setting = 271
)

// Define the struct to unmarshal the response
type ReadSettingResponse struct {
	Data struct {
		Value interface{} `json:"value"` // Value can be any type (string or number)
	} `json:"data"`
}

func (c *GivenergyClient) ReadSetting(setting Setting) (string, error) {
	// Define the URL for the API endpoint
	url := fmt.Sprintf(
		"%s/inverter/%s/settings/%d/read",
		c.ApiUrl,
		c.InverterId,
		setting)

	// Create a new request with the URL, method, and a nil body
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Create a new HTTP client and send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	log.Info().Str("body", string(body)).Msg("Got body")
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	// Check if the status code is OK
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf(
			"request failed with status code: %d\nError response: %s",
			resp.StatusCode, string(body))
	}

	// Unmarshal the response body into the ApiResponse struct
	var apiResp ReadSettingResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %v", err)
	}

	// Handle the value field which can be either a string or a number
	switch v := apiResp.Data.Value.(type) {
	case string:
		// If the value is a string, return it directly
		return v, nil
	case float64:
		// If the value is numeric (float64), convert it to string and return it
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	default:
		return "", fmt.Errorf("unexpected value type: %T", v)
	}
}

func (c *GivenergyClient) WriteSetting(setting Setting, value string) error {
	// Define the API endpoint
	url := fmt.Sprintf(
		"%s/inverter/%s/settings/%d/write",
		c.ApiUrl,
		c.InverterId,
		setting)

	// Create the payload
	payload := map[string]string{
		"value": value,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %v", err)
	}

	// Create a new HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Add headers
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Perform the HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Check the response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected response status: %s", resp.Status)
	}

	return nil
}

// Structs to match the JSON response structure
type InverterData struct {
	Data struct {
		Time   string `json:"time"`
		Status string `json:"status"`
		Solar  struct {
			Power  int `json:"power"`
			Arrays []struct {
				Array   int     `json:"array"`
				Voltage float64 `json:"voltage"`
				Current float64 `json:"current"`
				Power   int     `json:"power"`
			} `json:"arrays"`
		} `json:"solar"`
		Grid struct {
			Voltage   float64 `json:"voltage"`
			Current   float64 `json:"current"`
			Power     int     `json:"power"`
			Frequency float64 `json:"frequency"`
		} `json:"grid"`
		Battery struct {
			Percent     float64 `json:"percent"`
			Power       int     `json:"power"`
			Temperature float64 `json:"temperature"`
		} `json:"battery"`
		Inverter struct {
			Temperature     float64 `json:"temperature"`
			Power           int     `json:"power"`
			OutputVoltage   float64 `json:"output_voltage"`
			OutputFrequency float64 `json:"output_frequency"`
			EpsPower        int     `json:"eps_power"`
		} `json:"inverter"`
		Consumption int `json:"consumption"`
	} `json:"data"`
}

// GetInverterData function to fetch inverter data
func (c *GivenergyClient) GetInverterData() (*InverterData, error) {
	// Define the URL for the API endpoint
	url := fmt.Sprintf("%s/inverter/%s/system-data/latest", c.ApiUrl, c.InverterId)

	// Create a new request with the URL, method, and nil body
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Create a new HTTP client and send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	// Check if the status code is OK
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status code: %d\nError response: %s", resp.StatusCode, string(body))
	}

	// Unmarshal the response body into the InverterData struct
	var data InverterData
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	// Return the parsed data
	return &data, nil
}
