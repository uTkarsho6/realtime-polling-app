package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	gwTypes "github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi/types"
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

// SubmitVoteRequest is the expected JSON body.
type SubmitVoteRequest struct {
	Option string `json:"option"`
}

var (
	dbClient    *dynamodb.Client
	apiGwClient *apigatewaymanagementapi.Client
)

func init() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic("failed to load AWS config: " + err.Error())
	}
	dbClient = dynamodb.NewFromConfig(cfg)

	wsEndpoint := os.Getenv("WEBSOCKET_ENDPOINT")
	if wsEndpoint != "" {
		apiGwClient = apigatewaymanagementapi.NewFromConfig(cfg, func(o *apigatewaymanagementapi.Options) {
			o.BaseEndpoint = aws.String(wsEndpoint)
		})
	}
}

func getApiGwClient(ctx context.Context) *apigatewaymanagementapi.Client {
	if apiGwClient != nil {
		return apiGwClient
	}
	wsEndpoint := os.Getenv("WEBSOCKET_ENDPOINT")
	if wsEndpoint == "" {
		return nil
	}
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil
	}
	apiGwClient = apigatewaymanagementapi.NewFromConfig(cfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = aws.String(wsEndpoint)
	})
	return apiGwClient
}

func broadcastUpdate(ctx context.Context, pollId string, poll PollItem) {
	client := getApiGwClient(ctx)
	if client == nil {
		return
	}

	// Query all connections watching this poll from the GSI
	queryOut, err := dbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(os.Getenv("CONNECTIONS_TABLE")),
		IndexName:              aws.String("PollIdIndex"),
		KeyConditionExpression: aws.String("PollId = :pollId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pollId": &types.AttributeValueMemberS{Value: pollId},
		},
	})
	if err != nil {
		return
	}

	payload, err := json.Marshal(map[string]interface{}{
		"pollId":   poll.PollId,
		"question": poll.Question,
		"options":  poll.Options,
	})
	if err != nil {
		return
	}

	// Push update to every connected client
	for _, item := range queryOut.Items {
		connIdAttr, ok := item["ConnectionId"].(*types.AttributeValueMemberS)
		if !ok || connIdAttr.Value == "" {
			continue
		}
		connId := connIdAttr.Value

		_, err := client.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
			ConnectionId: aws.String(connId),
			Data:         payload,
		})
		if err != nil {
			var goneErr *gwTypes.GoneException
			if errors.As(err, &goneErr) {
				// Client disconnected without clean disconnect -> delete stale record
				_, _ = dbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
					TableName: aws.String(os.Getenv("CONNECTIONS_TABLE")),
					Key: map[string]types.AttributeValue{
						"ConnectionId": &types.AttributeValueMemberS{Value: connId},
					},
				})
			}
		}
	}
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

	// Atomically increment the vote count and return the updated poll attributes
	updateOut, err := dbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(os.Getenv("POLLS_TABLE")),
		Key: map[string]types.AttributeValue{
			"PollId": &types.AttributeValueMemberS{Value: pollId},
		},
		UpdateExpression:    aws.String("ADD Options.#opt.#cnt :one"),
		ConditionExpression: aws.String("attribute_exists(PollId)"),
		ReturnValues:        types.ReturnValueAllNew,
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

	// Unmarshal updated poll and broadcast to live WebSocket connections
	var updatedPoll PollItem
	if err := attributevalue.UnmarshalMap(updateOut.Attributes, &updatedPoll); err == nil {
		broadcastUpdate(ctx, pollId, updatedPoll)
	}

	// Success response to the voter
	resp, _ := json.Marshal(map[string]string{"message": "vote recorded"})
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(resp),
		Headers: map[string]string{
			"Content-Type":                 "application/json",
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "GET,POST,OPTIONS",
			"Access-Control-Allow-Headers": "Content-Type,X-Amz-Date,Authorization,X-Api-Key,X-Amz-Security-Token",
		},
	}, nil
}

func errorResponse(status int, message string) events.APIGatewayProxyResponse {
	body, _ := json.Marshal(map[string]string{"error": message})
	return events.APIGatewayProxyResponse{
		StatusCode: status,
		Body:       string(body),
		Headers: map[string]string{
			"Content-Type":                 "application/json",
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "GET,POST,OPTIONS",
			"Access-Control-Allow-Headers": "Content-Type,X-Amz-Date,Authorization,X-Api-Key,X-Amz-Security-Token",
		},
	}
}

func main() {
	lambda.Start(handler)
}
