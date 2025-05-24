package main

import (
	"bytes"
	"context"
	"fmt"
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
	getMetadataPath = "GET /rb/metadata"
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
		default:
			return events.APIGatewayV2HTTPResponse{
				Body:       "Path Not Found",
				StatusCode: http.StatusNotFound,
			}, nil
		}
	}()
}

func getMetadata() (events.APIGatewayV2HTTPResponse, error) {
	metadata, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(metadataBucketName),
		Key:    aws.String(metadataFileKey),
	})
	if err != nil {
		log.Println(fmt.Sprintf("Error getting object from S3: %v", err))
		return events.APIGatewayV2HTTPResponse{
			Body:       "Error getting metadata",
			StatusCode: http.StatusInternalServerError,
		}, nil
	}

	defer func() {
		if err := metadata.Body.Close(); err != nil {
			log.Printf("error closing S3 body: %v", err)
		}
	}()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, metadata.Body)
	if err != nil {
		log.Println(fmt.Sprintf("Error reading S3 content: %v", err))
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

func main() {
	lambda.Start(handler)
}
