package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// SubmitVoteRequest is the expected JSON body.
type SubmitVoteRequest struct {
	Option string `json:"option"`
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
	// Parse request body
	var body SubmitVoteRequest
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return errorResponse(http.StatusBadRequest, "invalid JSON"), nil
	}
	if body.Option == "" {
		return errorResponse(http.StatusBadRequest, "option is required"), nil
	}
	// Atomically increment the vote count
	_, err := dbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(os.Getenv("POLLS_TABLE")),
		Key: map[string]types.AttributeValue{
			"PollId": &types.AttributeValueMemberS{Value: pollId},
		},
		UpdateExpression:    aws.String("ADD Options.#opt.#cnt :one"),
		ConditionExpression: aws.String("attribute_exists(PollId)"),
		ExpressionAttributeNames: map[string]string{
			"#opt": body.Option,
			"#cnt": "count",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":one": &types.AttributeValueMemberN{Value: "1"},
		},
	})
	if err != nil {
		// Poll does not exist
		var condErr *types.ConditionalCheckFailedException
		if errors.As(err, &condErr) {
			return errorResponse(http.StatusNotFound, "poll not found"), nil
		}
		return errorResponse(http.StatusInternalServerError, "failed to record vote"), nil
	}
	// Success
	resp, _ := json.Marshal(map[string]string{"message": "vote recorded"})
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
