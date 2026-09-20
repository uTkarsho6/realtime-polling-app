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

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// ConnectionItem is the DynamoDB record for an active WebSocket connection.
type ConnectionItem struct {
	ConnectionId string `dynamodbav:"ConnectionId"`
	PollId       string `dynamodbav:"PollId"`
	ConnectedAt  string `dynamodbav:"ConnectedAt"`
}

var dbClient *dynamodb.Client

func init() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic("failed to load AWS config: " + err.Error())
	}
	dbClient = dynamodb.NewFromConfig(cfg)
}

func handler(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	connectionId := req.RequestContext.ConnectionID
	pollId := req.QueryStringParameters["pollId"]

	if connectionId == "" || pollId == "" {
		return errorResponse(http.StatusBadRequest, "connectionId and pollId are required"), nil
	}

	item := ConnectionItem{
		ConnectionId: connectionId,
		PollId:       pollId,
		ConnectedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "failed to marshal item"), nil
	}

	_, err = dbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(os.Getenv("CONNECTIONS_TABLE")),
		Item:      av,
	})
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "failed to save connection"), nil
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK}, nil
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
