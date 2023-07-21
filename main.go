package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/disintegration/imaging"
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
var s3Client *s3.S3
var allHeaders map[string]string

func init() {
	// Initialize the map during program initialization
	allHeaders = map[string]string{
		"Content-Type":                 "application/json",
		"Access-Control-Allow-Methods": "*",
		"Access-Control-Allow-Origin":  "*",
	}
	// Initialize AWS session and DynamoDB client
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1"), // Replace with your desired region
	})

	if err != nil {
		log.Fatal("Failed to create AWS session:", err)
	}

	dynamoDBClient = dynamodb.New(sess)
	s3Client = s3.New(sess)
}

func generateThumbnail(ctx context.Context, product Product) ([]byte, error) {
	// Fetch the image from the provided URL
	resp, err := http.Get(product.ImageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch product image: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch product image, status code: %d", resp.StatusCode)
	}

	imageData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read product image data: %v", err)
	}

	// Decode imageData to image.Image
	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode product image: %v", err)
	}

	// Generate a thumbnail (e.g., 100x100 pixels) using imaging library
	thumbnailImage := imaging.Resize(img, 100, 100, imaging.Lanczos)

	// Encode the thumbnail image to a byte slice
	var thumbnailData bytes.Buffer
	err = jpeg.Encode(&thumbnailData, thumbnailImage, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to encode thumbnail image: %v", err)
	}

	return thumbnailData.Bytes(), nil
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

func createProductHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var product Product
	if err := json.Unmarshal([]byte(request.Body), &product); err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Headers:    allHeaders,
			Body:       `{"error": "Invalid request payload"}`,
		}, nil
	}

	if product.Name == "" || product.Price <= 0 {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Headers:    allHeaders,
			Body:       `{"error": "Invalid product data. Name and Price are required fields"}`,
		}, nil
	}

	createdProduct, err := createProduct(product)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Headers:    allHeaders,
			Body:       `{"error": "Error creating product"}`,
		}, nil
	}

	// Launch a goroutine to generate the thumbnail and upload it to S3 asynchronously
	// go func() {
	log.Println("Generating thumbnail")
	thumbnail, err := generateThumbnail(ctx, product)
	if err != nil {
		log.Printf("Error generating thumbnail for product ID %s: %v", product.ID, err)
	}

	// Upload the thumbnail to S3
	s3Client := s3.New(session.Must(session.NewSession(&aws.Config{
		Region: aws.String("us-east-1"),
	})))
	thumbnailFileName := fmt.Sprintf("thumbnails/%s_thumbnail.jpg", product.ID)
	_, err = s3Client.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket:      aws.String("prodcut-thumbnails-sakar"),
		Key:         aws.String(thumbnailFileName),
		ContentType: aws.String("image/jpeg"),
		Body:        bytes.NewReader(thumbnail),
		ACL:         aws.String("public-read"),
	})
	if err != nil {
		log.Printf("Error uploading thumbnail to S3 for product ID %s: %v", product.ID, err)
	}
	log.Printf("Completed Generating thumbnail, %v", thumbnailFileName)
	// }()

	responseBody, _ := json.Marshal(createdProduct)
	return events.APIGatewayProxyResponse{
		StatusCode: 201,
		Headers:    allHeaders,
		Body:       string(responseBody),
	}, nil
}

func listProducts() ([]Product, error) {
	// Scan the entire table to get all products
	input := &dynamodb.ScanInput{
		TableName: aws.String("ProductCatalog"),
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
		TableName: aws.String("ProductCatalog"),
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(productID),
			},
		},
	}

	_, err := dynamoDBClient.DeleteItem(input)
	return err
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	switch request.HTTPMethod {
	case "POST":
		// Validate request body for POST (createProduct) request
		if request.Body == "" {
			return events.APIGatewayProxyResponse{
				StatusCode: 400,
				Headers:    allHeaders,
				Body:       `{"error": "Empty request body"}`,
			}, nil
		}

		return createProductHandler(ctx, request)

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
			Headers:    allHeaders,
			Body:       string(responseBody),
		}, nil

	case "DELETE":
		// Delete the product
		productID := request.PathParameters["productID"]
		if productID == "" {
			return events.APIGatewayProxyResponse{
				StatusCode: 400,
				Headers:    allHeaders,
				Body:       `{"error": "Invalid product ID"}`,
			}, nil
		}
		if err := deleteProduct(productID); err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 500,
				Headers:    allHeaders,
				Body:       fmt.Sprintf("{\"error\": \"Empty request body: %v \"}\n", err),
			}, nil
		}

		return events.APIGatewayProxyResponse{
			StatusCode: 204,
			Headers:    allHeaders,
			Body:       `{"message": "Deleted Successfully"}`,
		}, nil

	default:
		return events.APIGatewayProxyResponse{
			StatusCode: 405,
			Headers:    allHeaders,
			Body:       `{"error": "Method not allowed"}`,
		}, nil
	}
}

func main() {
	lambda.Start(handler)
}
