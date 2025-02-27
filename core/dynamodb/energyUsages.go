package dynamodb

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/jtfm/smartcharge/core/givenergy"
	"github.com/jtfm/smartcharge/core/utils"
)

// Reads energy usages from the DynamoDB table
func (c *ddbClient) ReadEnergyUsages(ctx context.Context, start time.Time, end time.Time) ([]givenergy.EnergyUsage, error) {
	tableName := utils.GetEnvStrict("DDB_ENERGY_USAGES_TABLE_NAME")

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
	}

	result, err := c.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query DynamoDB table: %w", err)
	}

	energyUsages := make([]givenergy.EnergyUsage, 0, len(result.Items))
	for _, item := range result.Items {
		energyUsage, err := c.toEnergyUsage(item)
		if err != nil {
			return nil, err
		}

		energyUsages = append(energyUsages, *energyUsage)
	}

	return energyUsages, nil
}

// Converts a map of DynamoDB attribute values to an EnergyUsage struct
func (c *ddbClient) toEnergyUsage(item map[string]types.AttributeValue) (*givenergy.EnergyUsage, error) {
	// Convert the DynamoDB attribute values to EnergyUsage fields
	var eu givenergy.EnergyUsage
	if item["start_time"] != nil {
		startTimeUnix, err := strconv.ParseInt(item["start_time"].(*types.AttributeValueMemberN).Value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to convert start time to int: %v", err)
		}
		eu.StartTime = time.Unix(startTimeUnix, 0)
	}

	if item["end_time"] != nil {
		endTimeUnix, err := strconv.ParseInt(item["end_time"].(*types.AttributeValueMemberN).Value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to convert end time to int: %v", err)
		}
		eu.EndTime = time.Unix(endTimeUnix, 0)
	}

	if item["energy_usage"] != nil {
		energyUsage, err := strconv.ParseFloat(item["energy_usage"].(*types.AttributeValueMemberN).Value, 64)
		if err != nil {
			return nil, err // Handle the error appropriately
		}
		eu.EnergyUsage = float32(energyUsage) // Convert from float64 to float32
	}

	return &eu, nil
}

// Writes energy usages to the DynamoDB table
func (c *ddbClient) WriteEnergyUsages(ctx context.Context, energyUsages []givenergy.EnergyUsage) error {
	var writeRequests []types.WriteRequest

	if energyUsages == nil {
		return fmt.Errorf("no energy usages to store")
	}

	// If there are no energy usages, return immediately
	if len(energyUsages) == 0 {
		return nil
	}

	for _, energyUsage := range energyUsages {
		if energyUsage.StartTime.IsZero() || energyUsage.EndTime.IsZero() {
			return fmt.Errorf("invalid energy usage: %+v", energyUsage)
		}
		writeRequests = append(writeRequests, c.energyUsageToWriteRequest(&energyUsage))
	}

	// If there are no write requests, return immediately
	if len(writeRequests) == 0 {
		return nil
	}

	tableName := utils.GetEnvStrict("DDB_ENERGY_USAGES_TABLE_NAME")

	_, err := c.batchWriteItems(ctx, writeRequests, tableName)
	if err != nil {
		return err
	}

	return nil
}

// Converts an EnergyUsage to a DynamoDB WriteRequest
func (c *ddbClient) energyUsageToWriteRequest(energyUsage *givenergy.EnergyUsage) types.WriteRequest {
	// Convert the EnergyUsage fields to DynamoDB attribute values
	item := map[string]types.AttributeValue{
		"partition_key": &types.AttributeValueMemberN{Value: "1"},
		"start_time": &types.AttributeValueMemberN{
			Value: fmt.Sprintf("%d", energyUsage.StartTime.Unix()), // Convert to Unix timestamp
		},
		"end_time": &types.AttributeValueMemberN{
			Value: fmt.Sprintf("%d", energyUsage.EndTime.Unix()), // Convert to Unix timestamp
		},
		"energy_usage": &types.AttributeValueMemberN{
			Value: fmt.Sprintf("%f", energyUsage.EnergyUsage),
		},
	}

	return types.WriteRequest{
		PutRequest: &types.PutRequest{
			Item: item,
		},
	}
}
