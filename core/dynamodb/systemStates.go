package dynamodb

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rs/zerolog/log"

	"github.com/jtfm/smartcharge/core/utils"
)

// SystemState represents the state of the smartcharge system at a given time
// It includes the time, the estimated PV generation, the predicted energy usage,  the predicted battery power, the unit rate, and whether the battery is planned to charge
type SystemState struct {
	StartTime             time.Time `json:"start_time"`
	PvEstimate            *float64  `json:"pv_estimate"`
	PredictedUsage        *float64  `json:"predicted_usage"`
	PredictedBatteryPower *float64  `json:"predicted_battery_power"`
	UnitRate              *float64  `json:"unit_rate"`
	WillCharge            bool      `json:"will_charge"`
}

// Reads system states from the DynamoDB table
func (c *ddbClient) ReadSystemStates(ctx context.Context, start, end time.Time) (
	[]SystemState, error) {
	tableName := utils.GetEnvStrict("DDB_SYSTEM_STATES_TABLE_NAME")

	var systemStates []SystemState
	var lastEvaluatedKey map[string]types.AttributeValue

	for {
		input := &dynamodb.QueryInput{
			TableName: &tableName,
			KeyConditions: map[string]types.Condition{
				"partition_key": { // Fixed value
					ComparisonOperator: types.ComparisonOperatorEq,
					AttributeValueList: []types.AttributeValue{
						&types.AttributeValueMemberN{Value: "1"},
					},
				},
				"start_time": {
					ComparisonOperator: types.ComparisonOperatorBetween,
					AttributeValueList: []types.AttributeValue{
						&types.AttributeValueMemberN{Value: fmt.Sprintf("%d", start.Unix())},
						&types.AttributeValueMemberN{Value: fmt.Sprintf("%d", end.Unix())},
					},
				},
			},
			ExclusiveStartKey: lastEvaluatedKey, // Continue from last result
		}

		result, err := c.client.Query(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to query DynamoDB table: %w", err)
		}

		for _, item := range result.Items {
			systemState, err := c.attributeValuesToSystemState(item)
			if err != nil {
				return nil, err
			}
			systemStates = append(systemStates, *systemState)
		}

		// Check if there are more results
		if result.LastEvaluatedKey == nil {
			break
		}
		lastEvaluatedKey = result.LastEvaluatedKey
	}

	return systemStates, nil
}

// Converts a map of DynamoDB attribute values to a SystemState struct
func (c *ddbClient) attributeValuesToSystemState(item map[string]types.AttributeValue) (*SystemState, error) {
	// Convert the DynamoDB attribute values to EnergyUsage fields
	var s SystemState
	if item["start_time"] != nil {
		startTimeUnix, err := strconv.ParseInt(item["start_time"].(*types.AttributeValueMemberN).Value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to convert start time to int: %v", err)
		}
		s.StartTime = time.Unix(startTimeUnix, 0)
	}

	if item["pv_estimate"] != nil {
		pvEstimate, err := strconv.ParseFloat(item["pv_estimate"].(*types.AttributeValueMemberN).Value, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to convert pv estimate to float: %v", err)
		}
		s.PvEstimate = &pvEstimate
	}

	if item["predicted_usage"] != nil {
		predictedUsage, err := strconv.ParseFloat(item["predicted_usage"].(*types.AttributeValueMemberN).Value, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to convert predicted energy usage to float: %v", err)
		}
		s.PredictedUsage = &predictedUsage
	}

	if item["predicted_battery_power"] != nil {
		predictedBatteryPower, err := strconv.ParseFloat(item["predicted_battery_power"].(*types.AttributeValueMemberN).Value, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to convert predicted battery power to float: %v", err)
		}
		s.PredictedBatteryPower = &predictedBatteryPower
	}

	if item["unit_rate"] != nil {
		unitRate, err := strconv.ParseFloat(item["unit_rate"].(*types.AttributeValueMemberN).Value, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to convert unit rate to float: %v", err)
		}
		s.UnitRate = &unitRate
	}

	if item["will_charge"] != nil {
		s.WillCharge = item["will_charge"].(*types.AttributeValueMemberBOOL).Value
	}

	return &s, nil
}

// Writes the given system states to the DynamoDB table
func (c *ddbClient) WriteSystemStates(ctx context.Context, systemStates []SystemState) error {

	if len(systemStates) == 0 {
		return nil
	}

	var writeRequests []types.WriteRequest
	for _, systemState := range systemStates {
		writeRequests = append(writeRequests, c.systemStateToWriteRequest(&systemState))
	}

	// If there are no write requests, return immediately
	if len(writeRequests) == 0 {
		return nil
	}

	tableName := utils.GetEnvStrict("DDB_SYSTEM_STATES_TABLE_NAME")

	_, err := c.batchWriteItems(ctx, writeRequests, tableName)
	if err != nil {
		return err
	}

	log.Info().Msg("Written system states to database.")

	return nil
}

func (c *ddbClient) systemStateToWriteRequest(s *SystemState) types.WriteRequest {
	attributeMap := map[string]types.AttributeValue{
		"partition_key": &types.AttributeValueMemberN{Value: "1"},
		"start_time": &types.AttributeValueMemberN{
			Value: fmt.Sprintf("%d", s.StartTime.Unix())},
	}

	if s.PvEstimate != nil {
		attributeMap["pv_estimate"] = &types.AttributeValueMemberN{
			Value: fmt.Sprintf("%f", *s.PvEstimate)}
	}
	if s.PredictedUsage != nil {
		attributeMap["predicted_usage"] = &types.AttributeValueMemberN{
			Value: fmt.Sprintf("%f", *s.PredictedUsage)}
	}
	if s.PredictedBatteryPower != nil {
		attributeMap["predicted_battery_power"] = &types.AttributeValueMemberN{
			Value: fmt.Sprintf("%f", *s.PredictedBatteryPower)}
	}
	if s.UnitRate != nil {
		attributeMap["unit_rate"] = &types.AttributeValueMemberN{
			Value: fmt.Sprintf("%f", *s.UnitRate)}
	}
	attributeMap["will_charge"] = &types.AttributeValueMemberBOOL{Value: s.WillCharge}

	return types.WriteRequest{
		PutRequest: &types.PutRequest{
			Item: attributeMap,
		},
	}
}
