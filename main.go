package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/google/uuid"
)

type Product struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Price       int    `json:"price"`
	ImageURL    string `json:"image_url"`
}

var dynamoDBClient *dynamodb.DynamoDB

func init() {
	// Initialize AWS session and DynamoDB client
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1"), // Replace with your desired region
	})

	if err != nil {
		log.Fatal("Failed to create AWS session:", err)
	}

	dynamoDBClient = dynamodb.New(sess)
}

func createProduct(product Product) (*Product, error) {
	// Generate a new UUID as the product ID
	product.ID = uuid.New().String()

	// Convert Product struct to DynamoDB AttributeValue
	item, err := dynamodbattribute.MarshalMap(product)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal product: %v", err)
	}

	// Create the item in DynamoDB
	input := &dynamodb.PutItemInput{
		Item:      item,
		TableName: aws.String("ProductCatalog"), // Replace with your DynamoDB table name
	}

	_, err = dynamoDBClient.PutItem(input)
	if err != nil {
		return nil, fmt.Errorf("failed to create product: %v", err)
	}

	return &product, nil
}

func listProducts() ([]Product, error) {
	// Scan the entire table to get all products
	input := &dynamodb.ScanInput{
		TableName: aws.String("ProductCatalog"), // Replace with your DynamoDB table name
	}

	result, err := dynamoDBClient.Scan(input)
	if err != nil {
		return nil, fmt.Errorf("failed to scan products: %v", err)
	}

	// Unmarshal the DynamoDB items to Product structs
	var products []Product
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &products)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal products: %v", err)
	}

	return products, nil
}

func deleteProduct(productID string) error {
	// Delete the product from DynamoDB
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String("ProductCatalog"), // Replace with your DynamoDB table name
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(productID),
			},
		},
	}

	_, err := dynamoDBClient.DeleteItem(input)
	return err
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	switch request.HTTPMethod {
	case "POST":
		// Validate request body for POST (createProduct) request
		if request.Body == "" {
			return events.APIGatewayProxyResponse{
				StatusCode: 400,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       `{"error": "Empty request body"}`,
			}, nil
		}
		// Create a new product
		var product Product
		if err := json.Unmarshal([]byte(request.Body), &product); err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 400,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       `{"error": "Invalid request payload"}`,
			}, err
		}

		if product.Name == "" || product.Price <= 0 {
			return events.APIGatewayProxyResponse{
				StatusCode: 400,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       `{"error": "Invalid product data. Name and Price are required fields"}`,
			}, nil
		}

		createdProduct, err := createProduct(product)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 500,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       `{"error": "Error creating product"}`,
			}, err
		}

		responseBody, err := json.Marshal(createdProduct)
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 500}, err
		}

		return events.APIGatewayProxyResponse{
			StatusCode: 201,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       string(responseBody),
		}, nil
	case "GET":
		// List all products
		products, err := listProducts()
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 500}, err
		}

		responseBody, err := json.Marshal(products)
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 500}, err
		}

		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       string(responseBody),
		}, nil
	case "DELETE":
		// Delete the product
		productID := request.PathParameters["productID"]
		if err := deleteProduct(productID); err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 500}, err
		}

		return events.APIGatewayProxyResponse{StatusCode: 204}, nil
	default:
		return events.APIGatewayProxyResponse{StatusCode: 405}, nil
	}
}

func main() {
	lambda.Start(handler)
}
