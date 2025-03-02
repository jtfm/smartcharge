package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jtfm/smartcharge/core/dynamodb"
	"github.com/jtfm/smartcharge/core/givenergy"
	"github.com/jtfm/smartcharge/core/octogql"
	"github.com/jtfm/smartcharge/core/solcast"
	"github.com/jtfm/smartcharge/core/utils"
	"github.com/rs/zerolog/log"
)

var ddbClient = dynamodb.InitDbClient(context.Background())

var geClient = givenergy.NewGivenergyClient(
	utils.GetEnvStrict("GIVENERGY_TOKEN"),
	utils.GetEnvStrict("GIVENERGY_INVERTER_ID"),
)

func main() {

	ctx := context.Background()

	if isInLambda() {
		lambda.Start(handler)
	} else {
		// Local development
		err := handler(ctx)
		if err != nil {
			log.Fatal().Err(err).Msg("Error calling handler")
		}
	}
}

func isInLambda() bool {
	return os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != ""
}

func handler(ctx context.Context) error {

	_, err := updateEnergyUsage(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Error updating energy usage")
		return err
	}

	latestHalfHour := time.Now().Truncate(30 * time.Minute)
	systemStates, err := ddbClient.ReadSystemStates(
		ctx, latestHalfHour, latestHalfHour.Add(24*time.Hour))
	if err != nil {
		return err
	}

	if len(systemStates) > 48 {
		log.Error().Msgf("Found %d system states. Expected 48.", len(systemStates))
	}

	if len(systemStates) < 48 {
		log.Info().Msgf("Found %d system states", len(systemStates))

		pvEstimate := 0.0
		for i := len(systemStates); i < 48; i++ {
			systemStateTime := latestHalfHour.Add(time.Duration(i) * 30 * time.Minute)
			log.Info().Msgf("Creating new system state for %s", systemStateTime.Format("2006-01-02 15:04:05"))
			systemStates = append(systemStates, dynamodb.SystemState{
				StartTime:  systemStateTime,
				PvEstimate: &pvEstimate,
			})
		}
	}

	forecastRequestHour := 0
	forecastRequestMinute := 30

	// Get forecasts for the next 24 hours at 00:30
	log.Info().Msgf("Latest half hour: %s", latestHalfHour.Format("2006-01-02 15:04:05"))

	forecastsRequired := false
	if latestHalfHour.Hour() == forecastRequestHour && latestHalfHour.Minute() >= forecastRequestMinute {
		forecastsRequired = true
	}

	if forecastsRequired {
		log.Info().Msg("Getting solar forecasts from API...")
		solCastClient := solcast.NewSolcastClient(
			utils.GetEnvStrict("SOLCAST_API_KEY"), utils.GetEnvStrict("SOLCAST_SITE_CODE"))
		forecasts, err := solCastClient.GetSolarForecasts()
		if err != nil {
			log.Error().Err(err).Msg("Error getting solar forecasts")
		}
		if forecasts != nil {
			for i := range systemStates {
				systemStates[i].PvEstimate = nil
				for _, forecast := range *forecasts {
					if systemStates[i].StartTime.Equal(
						forecast.StartTime) {
						pvEstimateCopy := forecast.PvEstimate
						systemStates[i].PvEstimate = &pvEstimateCopy
					}
				}
			}
			log.Info().Msgf("Got %d solar forecasts", len(*forecasts))

			// Write the forecasts to the database
			err = ddbClient.WriteSystemStates(ctx, systemStates)
			if err != nil {
				log.Error().Err(err).Msg("Error writing solar forecasts to the database")
				return err
			}
		}
	} else {
		log.Info().Msg("Not getting solar forecasts.")
	}

	updateUnitRatesRequired := false
	for _, systemState := range systemStates {
		if !updateUnitRatesRequired && systemState.UnitRate == nil {
			log.Info().Msgf(
				"Unit rate is nil at %s", systemState.StartTime.Format("2006-01-02 15:04:05"))
			updateUnitRatesRequired = true
			break
		}
	}

	log.Info().Msgf("Unit rate update required: %t", updateUnitRatesRequired)

	if updateUnitRatesRequired {
		unitRates, err := updateUnitRates()
		if err != nil {
			log.Error().Err(err).Msg("Error updating unit rates")
			return err
		}

		for i := range systemStates {
			for _, unitRate := range unitRates {
				if systemStates[i].UnitRate == nil && systemStates[i].StartTime.Equal(unitRate.ValidFrom) {
					unitRateValueCopy := unitRate.Value
					systemStates[i].UnitRate = &unitRateValueCopy
				}
			}
		}
	}

	// Predict the energy usage for the next 24 hours based on the average energy usage for the last 7 days

	// Get the average energy usage for the last 7 days
	startTime := latestHalfHour.Add(-(7 * 24 * time.Hour))
	endTime := latestHalfHour
	log.Info().Msgf("Getting energy usage for the last 7 days from %s to %s", startTime, endTime)
	usages, err := ddbClient.ReadEnergyUsages(ctx, startTime, endTime)
	if err != nil {
		log.Error().Err(err).Msg("Error getting average energy usage")
		return err
	}

	for i := range systemStates {
		var usageValues []float64
		for _, u := range usages {
			if u.StartTime.Format("15:04:05") == systemStates[i].StartTime.Format("15:04:05") {
				usageValues = append(usageValues, float64(u.EnergyUsage))
			}
		}
		if len(usageValues) == 0 {
			error := fmt.Errorf("no usage values found for %s", systemStates[i].StartTime.Format("15:04:05"))
			log.Error().Err(error).Msg("Error getting average energy usage")
			return error
		}
		var sum float64 = 0
		for _, v := range usageValues {
			sum += v
		}
		predictedUsage := sum / float64(len(usageValues))
		systemStates[i].PredictedUsage = &predictedUsage

		// Reset the WillCharge field
		systemStates[i].WillCharge = false
	}

	inverterData, err := geClient.GetInverterData()
	if err != nil {
		log.Error().Err(err).Msg("Error getting battery charge power")
		return err
	}

	// Initialize the predictions slice
	batteryCapacityInKWh := 9.5
	predictedBatteryPower := batteryCapacityInKWh * inverterData.Data.Battery.Percent / 100.0
	systemStates[0].PredictedBatteryPower = &predictedBatteryPower
	log.Info().Msgf("Initial battery %%: %f %%", inverterData.Data.Battery.Percent)
	log.Info().Msgf("Initial battery power: %f kWh", predictedBatteryPower)

	// Add auto charge logic
	for i := range systemStates {
		if systemStates[i].UnitRate != nil && *systemStates[i].UnitRate <= 0.0 {
			systemStates[i].WillCharge = true
		}
	}

	var grid30mChargeCapacity float64 = 1.6
	retries := 0
	for {
		var nextEmptyTime *time.Time
		for i := 1; i < len(systemStates); i++ {
			nextEmptyTime = nil

			if systemStates[i-1].PvEstimate == nil {
				error := fmt.Errorf("previous.PvEstimate is nil")
				log.Error().Err(error).Msg("Error planning charging times")
				return error
			}
			previousPvEstimate := *systemStates[i-1].PvEstimate

			if systemStates[i-1].PredictedUsage == nil {
				error := fmt.Errorf("previous.PredictedUsage is nil")
				log.Error().Err(error).Msg("Error planning charging times")
				return error
			}
			previousPredictedUsage := *systemStates[i-1].PredictedUsage

			previousPredictedBatteryPower := *systemStates[i-1].PredictedBatteryPower
			predictedBatteryPower := previousPredictedBatteryPower + previousPvEstimate - previousPredictedUsage

			if predictedBatteryPower > batteryCapacityInKWh {
				predictedBatteryPower = batteryCapacityInKWh
			}

			if systemStates[i-1].WillCharge {
				predictedBatteryPower += grid30mChargeCapacity
			}

			systemStates[i].PredictedBatteryPower = &predictedBatteryPower

			log.Info().Msgf(
				"Predictions for %s: usage: %s, forecast: %s, battery: %s, unitRate: %s, charging: %t",
				systemStates[i].StartTime.Format("2006-01-02 15:04:05"),
				utils.FormatFloatPointer(systemStates[i].PredictedUsage),
				utils.FormatFloatPointer(systemStates[i].PvEstimate),
				utils.FormatFloatPointer(systemStates[i].PredictedBatteryPower),
				utils.FormatFloatPointer(systemStates[i].UnitRate),
				systemStates[i].WillCharge,
			)
			if *systemStates[i].PredictedBatteryPower <= 0 {
				nextEmptyTime = &systemStates[i].StartTime
				break
			}
		}

		if nextEmptyTime == nil {
			log.Info().Msg(
				"Battery will not be empty in the next 24 hours. Finished planning charging times.")
			break
		}

		log.Info().Msgf(
			"Battery will be empty at %s. Recharge needed.",
			nextEmptyTime.Format("2006-01-02 15:04:05"),
		)
		// Find the best time to charge the battery before it runs out
		// This is the time when the unit rate is the lowest
		minimumUnitRate := 1000.0
		minimumIndex := 0
		for i, s := range systemStates {
			if s.StartTime.Equal(*nextEmptyTime) {
				break
			}
			if s.UnitRate != nil && *s.UnitRate < minimumUnitRate && !s.WillCharge {
				minimumIndex = i
				minimumUnitRate = *s.UnitRate
			}
		}

		log.Info().Msgf(
			"The best time to charge the battery is at %s when the unit rate is %f",
			systemStates[minimumIndex].StartTime.Format("2006-01-02 15:04:05"),
			*systemStates[minimumIndex].UnitRate)
		systemStates[minimumIndex].WillCharge = true

		retries++
		if retries > 10 {
			log.Info().Msg("Too many retries. Exiting planning phase.")
			break
		}
	}

	err = ddbClient.WriteSystemStates(ctx, systemStates)
	if err != nil {
		return err
	}

	if systemStates[0].WillCharge {
		log.Info().Msg("WillCharge is true. Charging the battery")

		// Set the charge limit to 100%
		err = geClient.WriteSetting(givenergy.Setting_ACChargeUpperLimit, "100")
		if err != nil {
			log.Error().Err(err).Msg("Error setting charge limit")
			return err
		}

		// Set the charge end time to 30 minutes after the start time
		err = geClient.WriteSetting(
			givenergy.Setting_ACCharge1EndTime,
			systemStates[0].StartTime.Add(30*time.Minute).Format("15:04"),
		)
		if err != nil {
			log.Error().Err(err).Msg("Error setting charge end time")
			return err
		}

		// Set the charge start time to the latest half hour
		err = geClient.WriteSetting(
			givenergy.Setting_ACCharge1StartTime,
			systemStates[0].StartTime.Format("15:04"))
		if err != nil {
			log.Error().Err(err).Msg("Error setting charge start time")
			return err
		}

		// Enable charging
		err = geClient.WriteSetting(givenergy.Setting_ACChargeEnable, "1")
		if err != nil {
			log.Error().Err(err).Msg("Error enabling charging")
			return err
		}
	} else {
		log.Info().Msg("WillCharge is false. Ensuring the battery is not charging")
		err = geClient.WriteSetting(givenergy.Setting_ACChargeEnable, "0")
		if err != nil {
			log.Error().Err(err).Msg("Error disabling charging")
			return err
		}
	}

	return nil
}

func updateUnitRates() ([]octogql.UnitRate, error) {
	client := octogql.NewOctopusClient(
		utils.GetEnvStrict("OCTOPUS_USERNAME"), utils.GetEnvStrict("OCTOPUS_PASSWORD"))

	octopusAccountNumber := utils.GetEnvStrict("OCTOPUS_ACCOUNT_NUMBER")
	unitRates, err := client.GetUnitRates(octopusAccountNumber)
	if err != nil {
		log.Error().Err(err).Msg("Error querying Octopus API unit rates")
		return nil, err
	}

	log.Info().Msgf("Found %d unit rates from Octopus API", len(unitRates))

	if len(unitRates) == 0 {
		err = fmt.Errorf("no unit rates found from Octopus API")
		log.Error().Err(err).Msg("Error getting unit rates")
		return nil, err
	}
	return unitRates, nil
}

func updateEnergyUsage(ctx context.Context) ([]givenergy.EnergyUsage, error) {
	// Get energy usage data from the inverter for the last week
	energyUsages, err := geClient.ReadUsage(time.Now().Add(-24*time.Hour), time.Now())
	if err != nil {
		log.Error().Err(err).Msg("Error getting energy usage data")
		return nil, err
	}

	log.Info().Msgf("Got %d energy usage records from the inverter", len(energyUsages))

	err = ddbClient.WriteEnergyUsages(ctx, energyUsages)
	if err != nil {
		log.Error().Err(err).Msg("Error writing energy usage data to the database")
		return nil, err
	}

	log.Info().Msg("Energy usage data written to the database")
	return energyUsages, nil
}
