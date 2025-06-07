package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"function/models"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
	"log"
	"net/http"
	"os"
)

const (
	getMetadataPath    = "GET /rb/metadata"
	updateMetadataPath = "PUT /rb/metadata"
)

var (
	metadataBucketName string
	metadataFileKey    string

	s3Client *s3.Client
)

func init() {
	metadataBucketName = os.Getenv("METADATA_BUCKET_NAME")
	metadataFileKey = os.Getenv("METADATA_FILE_KEY")

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic(err)
	}

	s3Client = s3.NewFromConfig(cfg)
}

func handler(request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	path := request.RouteKey

	return func() (events.APIGatewayV2HTTPResponse, error) {
		switch path {
		case getMetadataPath:
			return getMetadata()
		case updateMetadataPath:
			return updateMetadata(request)
		default:
			return events.APIGatewayV2HTTPResponse{
				Body:       "Path Not Found",
				StatusCode: http.StatusNotFound,
			}, nil
		}
	}()
}

func getMetadata() (events.APIGatewayV2HTTPResponse, error) {
	buf, err := getMetadataFromS3()
	if err != nil {
		log.Println(fmt.Sprintf("Error getting metadata: %v", err))
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       "Error getting metadata",
		}, nil
	}

	return events.APIGatewayV2HTTPResponse{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       buf.String(),
	}, nil
}

func updateMetadata(request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	var metadata []models.Metadata
	if err := json.Unmarshal([]byte(request.Body), &metadata); err != nil {
		log.Println(fmt.Sprintf("Error unmarshalling metadata from request body: %v", err))
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       "Error updating metadata",
		}, nil
	}

	_, err := s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(metadataBucketName),
		Key:         aws.String(metadataFileKey),
		Body:        bytes.NewReader([]byte(request.Body)),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		log.Println(fmt.Sprintf("Error upload to s3: %v", err))
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       "Error updating metadata",
		}, nil
	}

	return events.APIGatewayV2HTTPResponse{
		StatusCode: http.StatusOK,
	}, nil
}

func getMetadataFromS3() (bytes.Buffer, error) {
	metadata, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(metadataBucketName),
		Key:    aws.String(metadataFileKey),
	})
	if err != nil {
		return bytes.Buffer{}, err
	}

	defer func() {
		if err := metadata.Body.Close(); err != nil {
			log.Printf("error closing S3 body: %v", err)
		}
	}()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, metadata.Body)
	if err != nil {
		return bytes.Buffer{}, err
	}

	return *buf, nil
}

func main() {
	lambda.Start(handler)
}
