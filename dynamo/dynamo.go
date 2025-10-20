package dynamo

import (
	"context"
	"fmt"
	"time"

	"coreheadlines/typesPkg"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type PublishedArticleRecord struct {
	GUID      string `dynamodbav:"guid"`      // Main table PK
	Timestamp int64  `dynamodbav:"timestamp"` // Main table SK
	TTL       int64  `dynamodbav:"ttl"`       // Time to live (optional, for auto-expiration)
}

func IsArticlePublished(ctx context.Context, db *dynamodb.Client, guid string) (bool, error) {
	result, err := db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String("coreheadlines_table"),
		KeyConditionExpression: aws.String("guid = :guid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":guid": &types.AttributeValueMemberS{Value: guid},
		},
		Select: types.SelectCount,
		Limit:  aws.Int32(1),
	})

	if err != nil {
		return false, fmt.Errorf("failed to query DynamoDB: %w", err)
	}

	return result.Count > 0, nil
}

func BatchMarkPublished(
	ctx context.Context,
	db *dynamodb.Client,
	articles []typesPkg.MainStruct,
) error {
	// build all WriteRequests
	var writes []types.WriteRequest
	now := time.Now()
	ttl := now.AddDate(1, 0, 0).Unix()

	for _, art := range articles {
		rec := PublishedArticleRecord{
			GUID:      art.GUID,
			Timestamp: now.Unix(),
			TTL:       ttl,
		}
		item, err := attributevalue.MarshalMap(rec)
		if err != nil {
			return fmt.Errorf("marshal record: %w", err)
		}
		writes = append(writes, types.WriteRequest{
			PutRequest: &types.PutRequest{Item: item},
		})
	}

	// max 25 per batch
	for i := 0; i < len(writes); i += 25 {
		end := min(i+25, len(writes))
		batch := writes[i:end]
		input := &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				"coreheadlines_table": batch,
			},
		}

		resp, err := db.BatchWriteItem(ctx, input)
		if err != nil {
			return fmt.Errorf("batch write failed: %w", err)
		}

		// retry unprocessed items if any
		if un := resp.UnprocessedItems["coreheadlines_table"]; len(un) > 0 {
			retryInput := &dynamodb.BatchWriteItemInput{
				RequestItems: map[string][]types.WriteRequest{
					"coreheadlines_table": un,
				},
			}
			if _, err := db.BatchWriteItem(ctx, retryInput); err != nil {
				return fmt.Errorf("retry unprocessed failed: %w", err)
			}
		}
	}

	return nil
}
