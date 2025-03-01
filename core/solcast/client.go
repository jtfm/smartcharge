package solcast

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/rs/zerolog/log"
)

type SolcastClient struct {
	ApiKey   string
	SiteCode string
	ApiUrl   string
}

func NewSolcastClient(apiKey string, siteCode string) *SolcastClient {
	return &SolcastClient{
		ApiKey:   apiKey,
		SiteCode: siteCode,
		ApiUrl:   "https://api.solcast.com.au",
	}
}

// GetSolarForecast gets the solar forecast
func (c *SolcastClient) GetSolarForecasts() (*[]InternalForecast, error) {

	url := fmt.Sprintf(
		"%s/rooftop_sites/%s/forecasts?format=json",
		c.ApiUrl,
		c.SiteCode,
	)

	log.Info().Msgf("Getting solar forecast from url %s", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("Error creating request")
	}

	apiKey := os.Getenv("SOLCAST_API_KEY")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("rate limit exceeded")
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("error getting solar forecast: %s", resp.Status)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	forecasts, err := UnmarshalForecasts(body)
	if err != nil {
		return nil, err
	}

	internalForecasts := make([]InternalForecast, 0, len(forecasts.Forecasts))
	for _, forecast := range forecasts.Forecasts {
		internalForecast, err := ToInternal(forecast)
		if err != nil {
			return nil, err
		}
		internalForecasts = append(internalForecasts, *internalForecast)
	}

	return &internalForecasts, nil
}
