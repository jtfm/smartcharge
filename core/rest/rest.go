package resr

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog/log"
)

func queryRestApi(apiKey string, accountNumber string) error {
	log.Info().Msg("Querying Octopus Energy API")

	// API key and password (password is often just an empty string or not required)

	// Encode API key and password in Base64
	auth := base64.StdEncoding.EncodeToString([]byte(apiKey))

	// Create a new HTTP request
	req, err := http.NewRequest("GET", fmt.Sprintf(
		"https://api.octopus.energy/v1/accounts/%s/", accountNumber), nil)
	if err != nil {
		return err
	}

	// Add the Authorization header with Basic Authentication
	req.Header.Add("Authorization", "Basic "+auth)

	// Create an HTTP client and send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Parse the response body
	accountResp, err := UnmarshalAccountResp(body)
	if err != nil {
		return err
	}

	// Pretty print the response
	indented, err := accountResp.MarshalIndent()
	if err != nil {
		return err
	}

	log.Info().Msgf("Account resp: %s", string(indented))

	log.Info().Msg("Finished querying Octopus Energy API")
	return nil
}
