package main

import (
	"context"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	// Import image processing library
)

type Product struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ImageURL string `json:"image_url"`
}

func handleRequest(ctx context.Context, event events.DynamoDBEvent) error {
	log.Println("Lambda is trigered")
	for _, record := range event.Records {
		log.Printf("EventName: %s, TableName: %s, EventID: %s\n", record.EventName, record.EventSourceArn, record.EventID)
		// Process the event data here.
	}
	return nil
}

func main() {
	lambda.Start(handleRequest)
}
