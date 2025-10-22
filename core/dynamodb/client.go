package dynamodb

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/jtfm/smartcharge/core/utils"
	"github.com/rs/zerolog/log"
)

type DdbClient struct {
	client *dynamodb.Client
}

// InitDbClient initializes the DynamoDB client
func InitDbClient(ctx context.Context) *DdbClient {
	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(utils.GetEnvStrict("AWS_REGION"))) // Change region as needed
	if err != nil {
		log.Fatal().Err(err).Msg("Error loading AWS configuration")
	}

	// Create a DynamoDB client
	return &DdbClient{
		client: dynamodb.NewFromConfig(cfg),
	}
}

// Writes writeRequests to the DynamoDB table in batches of 25
func (c *DdbClient) batchWriteItems(
	ctx context.Context, writeRequests []types.WriteRequest, tableName string) (bool, error) {
	var batchSize = 25
	for i := 0; i < len(writeRequests); i += batchSize {

		end := i + batchSize
		if end > len(writeRequests) {
			end = len(writeRequests)
		}
		batch := writeRequests[i:end]

		input := &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				tableName: batch,
			},
		}

		_, err := c.client.BatchWriteItem(ctx, input)
		if err != nil {
			return true, fmt.Errorf("failed to batch write items in DynamoDB: %w", err)
		}
	}
	return false, nil
}
