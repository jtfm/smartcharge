package octogql

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Kraken Structs
type KrakenTokenAuthenticationResponse struct {
	Data KrakenTokenData `json:"data"`
}

type KrakenTokenData struct {
	ObtainKrakenToken ObtainKrakenToken `json:"obtainKrakenToken"`
}

type ObtainKrakenToken struct {
	Token string `json:"token"`
}

// Account Structs
type AccountRoot struct {
	Data Data `json:"data"`
}

type Data struct {
	Account Account `json:"account"`
}

type Account struct {
	ElectricityAgreements []ElectricityAgreement `json:"electricityAgreements"`
}

type ElectricityAgreement struct {
	ID     int64  `json:"id"`
	Tariff Tariff `json:"tariff"`
}

type Tariff struct {
	ID             string     `json:"id"`
	DisplayName    string     `json:"displayName"`
	FullName       string     `json:"fullName"`
	ProductCode    string     `json:"productCode"`
	TariffCode     string     `json:"tariffCode"`
	StandingCharge float32    `json:"standingCharge"`
	UnitRates      []UnitRate `json:"unitRates"`
}

type UnitRate struct {
	ValidFrom time.Time `json:"validFrom"`
	ValidTo   time.Time `json:"validTo"`
	Value     float64   `json:"value"`
}

type OctopusClient struct {
	Token  string
	ApiUrl string
}

func NewOctopusClient(username string, password string) *OctopusClient {
	client := &OctopusClient{
		ApiUrl: "https://api.octopus.energy/v1/graphql/",
	}
	token, err := client.GetToken(username, password)
	if err != nil {
		log.Fatal().Err(err).Msg("Error getting token")
	}
	client.Token = *token

	return client
}

func (c *OctopusClient) GetToken(username string, password string) (*string, error) {
	query := `
mutation krakenTokenAuthentication($email: String!, $password: String!) {
  obtainKrakenToken(input: {email: $email, password: $password}) {
    token
  }
}`

	variables := map[string]string{
		"email":    username, // Replace with actual email
		"password": password, // Replace with actual password
	}

	reqBody := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}

	reqBodyJson, err := json.Marshal(reqBody)
	if err != nil {
		log.Fatal().Err(err).Msg("Error marshaling request body")
		return nil, err
	}

	req, err := http.NewRequest(
		"POST", c.ApiUrl, strings.NewReader(string(reqBodyJson)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	response := KrakenTokenAuthenticationResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return &response.Data.ObtainKrakenToken.Token, nil
}

func (c *OctopusClient) GetUnitRates(accountNumber string) ([]UnitRate, error) {
	electricityAgreementQuery := `
query Account($accountNumber: String!) {
	account(accountNumber: $accountNumber) {
		electricityAgreements {
			id,
			tariff {
				... on HalfHourlyTariff {
					id,
					displayName,
					fullName,
					productCode,
					tariffCode,
					standingCharge,
					unitRates {
						validFrom,
						validTo,
						value
					}
				}
			}
		}
  }
}`

	reqBody := map[string]interface{}{
		"query":     electricityAgreementQuery,
		"variables": map[string]string{"accountNumber": accountNumber},
	}

	reqBodyJson, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(
		"POST", c.ApiUrl, strings.NewReader(string(reqBodyJson)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.Token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Deserialize JSON response
	accountRoot := AccountRoot{}
	if err := json.Unmarshal(body, &accountRoot); err != nil {
		return nil, err
	}

	// Serialize Tariff data for non-outgoing rates
	unitRates := make([]UnitRate, 0)
	for _, agreement := range accountRoot.Data.Account.ElectricityAgreements {
		if !strings.Contains(agreement.Tariff.ProductCode, "OUTGOING") {
			continue
		}
		unitRates = append(unitRates, agreement.Tariff.UnitRates...)
	}

	return unitRates, nil
}
