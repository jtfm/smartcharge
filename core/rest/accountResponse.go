package resr

import (
	"encoding/json"
	"time"
)

func UnmarshalAccountResp(data []byte) (AccountResponse, error) {
	var r AccountResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *AccountResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func (r *AccountResponse) MarshalIndent() ([]byte, error) {
	return json.Marshal(r)
}

type AccountResponse struct {
	Number     string     `json:"number"`
	Properties []Property `json:"properties"`
}

type Property struct {
	ID                     int64                   `json:"id"`
	MovedInAt              time.Time               `json:"moved_in_at"`
	MovedOutAt             interface{}             `json:"moved_out_at"`
	AddressLine1           string                  `json:"address_line_1"`
	AddressLine2           string                  `json:"address_line_2"`
	AddressLine3           string                  `json:"address_line_3"`
	Town                   string                  `json:"town"`
	County                 string                  `json:"county"`
	Postcode               string                  `json:"postcode"`
	ElectricityMeterPoints []ElectricityMeterPoint `json:"electricity_meter_points"`
	GasMeterPoints         []GasMeterPoint         `json:"gas_meter_points"`
}

type ElectricityMeterPoint struct {
	Mpan                string                       `json:"mpan"`
	ProfileClass        int64                        `json:"profile_class"`
	ConsumptionStandard int64                        `json:"consumption_standard"`
	Meters              []ElectricityMeterPointMeter `json:"meters"`
	Agreements          []Agreement                  `json:"agreements"`
	IsExport            bool                         `json:"is_export"`
}

type Agreement struct {
	TariffCode string     `json:"tariff_code"`
	ValidFrom  time.Time  `json:"valid_from"`
	ValidTo    *time.Time `json:"valid_to"`
}

type ElectricityMeterPointMeter struct {
	SerialNumber string     `json:"serial_number"`
	Registers    []Register `json:"registers"`
}

type Register struct {
	Identifier           string `json:"identifier"`
	Rate                 string `json:"rate"`
	IsSettlementRegister bool   `json:"is_settlement_register"`
}

type GasMeterPoint struct {
	Mprn                string               `json:"mprn"`
	ConsumptionStandard int64                `json:"consumption_standard"`
	Meters              []GasMeterPointMeter `json:"meters"`
	Agreements          []Agreement          `json:"agreements"`
}

type GasMeterPointMeter struct {
	SerialNumber string `json:"serial_number"`
}
