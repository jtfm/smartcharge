package givenergy

type GivenergyClient struct {
	Token      string
	InverterId string
	ApiUrl     string
}

func NewGivenergyClient(token, inverterId string) *GivenergyClient {
	return &GivenergyClient{
		Token:      token,
		InverterId: inverterId,
		ApiUrl:     "https://api.givenergy.cloud/v1",
	}
}
