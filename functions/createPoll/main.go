package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/google/uuid"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// CreatePollRequest  expected JSON body
type CreatePollRequest struct {
	Question string   `json:"question"`
	Options  []string `json:"options"`
}

// what we return on success
type CreatePollResponse struct {
	PollId string `json:"pollId"`
}

// OptionCount holds the vote count for a single poll option in DynamoDB.
type OptionCount struct {
	Count int `dynamodbav:"count"`
}

// Build the DynamoDB item
type PollItem struct {
	PollId    string                 `dynamodbav:"PollId"`
	Question  string                 `dynamodbav:"Question"`
	Options   map[string]OptionCount `dynamodbav:"Options"`
	CreatedAt string                 `dynamodbav:"CreatedAt"`
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
	// Parse the request body
	var body CreatePollRequest
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return errorResponse(http.StatusBadRequest, "invalid JSON"), nil
	}

	if body.Question == "" {
		return errorResponse(http.StatusBadRequest, "question is required"), nil
	}
	if len(body.Options) < 2 || len(body.Options) > 5 {
		return errorResponse(http.StatusBadRequest, "provide between 2 and 5 options"), nil
	}

	for _, opt := range body.Options {
		if opt == "" {
			return errorResponse(http.StatusBadRequest, "options must not be empty"), nil
		}
	}
	// Generate unique poll ID
	pollId := uuid.New().String()

	// Build options map with zero counts
	options := make(map[string]OptionCount)
	for _, opt := range body.Options {
		options[opt] = OptionCount{Count: 0}
	}

	item := PollItem{
		PollId:    pollId,
		Question:  body.Question,
		Options:   options,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Marshal Go struct → DynamoDB attribute map
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "failed to marshal item"), nil
	}

	// Write item to DynamoDB
	_, err = dbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(os.Getenv("POLLS_TABLE")),
		Item:      av,
	})
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "failed to save poll"), nil
	}

	// Return 201 Created with the poll ID
	resp, _ := json.Marshal(CreatePollResponse{PollId: pollId})
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusCreated,
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
