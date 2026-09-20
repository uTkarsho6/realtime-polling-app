package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// OptionCount holds the vote count for a single poll option.
type OptionCount struct {
	Count int `dynamodbav:"count" json:"count"`
}

// PollItem is the DynamoDB record structure.
type PollItem struct {
	PollId    string                 `dynamodbav:"PollId"`
	Question  string                 `dynamodbav:"Question"`
	Options   map[string]OptionCount `dynamodbav:"Options"`
	CreatedAt string                 `dynamodbav:"CreatedAt"`
}

// GetPollResponse is what we return to the caller.
type GetPollResponse struct {
	PollId   string                 `json:"pollId"`
	Question string                 `json:"question"`
	Options  map[string]OptionCount `json:"options"`
}

var dbClient *dynamodb.Client

func init() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic("failed to load AWS config: " + err.Error())
	}
	dbClient = dynamodb.NewFromConfig(cfg)
}

func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Extract poll ID from URL path
	pollId := req.PathParameters["pollId"]
	if pollId == "" {
		return errorResponse(http.StatusBadRequest, "pollId is required"), nil
	}
	// Fetch poll from DynamoDB
	result, err := dbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(os.Getenv("POLLS_TABLE")),
		Key: map[string]types.AttributeValue{
			"PollId": &types.AttributeValueMemberS{Value: pollId},
		},
	})
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "failed to fetch poll"), nil
	}
	// Item not found
	if result.Item == nil {
		return errorResponse(http.StatusNotFound, "poll not found"), nil
	}
	// Unmarshal DynamoDB item → Go struct
	var poll PollItem
	if err := attributevalue.UnmarshalMap(result.Item, &poll); err != nil {
		return errorResponse(http.StatusInternalServerError, "failed to parse poll"), nil
	}
	// Build and return response
	resp, _ := json.Marshal(GetPollResponse{
		PollId:   poll.PollId,
		Question: poll.Question,
		Options:  poll.Options,
	})
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(resp),
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
}

func errorResponse(status int, message string) events.APIGatewayProxyResponse {
	body, _ := json.Marshal(map[string]string{"error": message})
	return events.APIGatewayProxyResponse{
		StatusCode: status,
		Body:       string(body),
		Headers:    map[string]string{"Content-Type": "application/json"},
	}
}
func main() {
	lambda.Start(handler)
}
