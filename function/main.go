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
	lambdaSDK "github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

const (
	getMetadataPath    = "GET /rb/metadata"
	updateMetadataPath = "PUT /rb/metadata"

	getQuestionCountPath = "POST /rb/questions/count"
)

var (
	metadataBucketName string
	metadataFileKey    string
	questionServiceArn string

	s3Client     *s3.Client
	lambdaClient *lambdaSDK.Client
)

func init() {
	metadataBucketName = os.Getenv("METADATA_BUCKET_NAME")
	metadataFileKey = os.Getenv("METADATA_FILE_KEY")
	questionServiceArn = os.Getenv("QUESTION_SERVICE_ARN")

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic(err)
	}

	s3Client = s3.NewFromConfig(cfg)
	lambdaClient = lambdaSDK.NewFromConfig(cfg)
}

func handler(request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	path := request.RouteKey

	return func() (events.APIGatewayV2HTTPResponse, error) {
		switch path {
		case getMetadataPath:
			return getMetadata()
		case updateMetadataPath:
			return updateMetadata()
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

func updateMetadata() (events.APIGatewayV2HTTPResponse, error) {
	fmt.Println("Update Metadata Begin")

	buf, err := getMetadataFromS3()
	if err != nil {
		log.Println(fmt.Sprintf("Error getting metadata: %v", err))
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       "Error getting metadata",
		}, nil
	}

	var metadataResp models.GetMetadataResponse
	err = json.Unmarshal(buf.Bytes(), &metadataResp)
	if err != nil {
		log.Println(fmt.Sprintf("Error updating metadata: %v", err))
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       "Error updating metadata",
		}, nil
	}

	var examIds []string
	for _, item := range metadataResp.Metadata {
		examIds = append(examIds, item.ExamId)
	}

	counts, err := getCountsFromService(examIds)
	if err != nil {
		log.Println(fmt.Sprintf("Error getting count from rb-question-service: %v", err))
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       "Error updating metadata",
		}, nil
	}

	updated := false
	for i := range metadataResp.Metadata {
		item := &metadataResp.Metadata[i]
		if count, exists := counts[item.ExamId]; exists && item.QuestionCount != count {
			item.QuestionCount = count
			updated = true
			fmt.Println(fmt.Sprintf("updating question count from %d to %d", item.QuestionCount, count))
		}
	}

	if updated {
		jsonBytes, err := json.MarshalIndent(metadataResp, "", "  ")
		if err != nil {
			log.Println(fmt.Sprintf("Error parsing json: %v", err))
			return events.APIGatewayV2HTTPResponse{
				StatusCode: http.StatusInternalServerError,
				Body:       "Error updating metadata",
			}, nil
		}

		_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket:      aws.String(metadataBucketName),
			Key:         aws.String(metadataFileKey),
			Body:        bytes.NewReader(jsonBytes),
			ContentType: aws.String("application/json"),
		})
		if err != nil {
			log.Println(fmt.Sprintf("Error upload to s3: %v", err))
			return events.APIGatewayV2HTTPResponse{
				StatusCode: http.StatusInternalServerError,
				Body:       "Error updating metadata",
			}, nil
		}
	}

	fmt.Println("Update Metadata End")

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

func getCountsFromService(examIds []string) (map[string]int, error) {
	payload := map[string]string{
		"routeKey": getQuestionCountPath,
		"body":     strings.Join(examIds, ","),
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return map[string]int{}, err
	}

	resp, err := lambdaClient.Invoke(context.TODO(), &lambdaSDK.InvokeInput{
		FunctionName:   aws.String(questionServiceArn),
		InvocationType: "RequestResponse",
		Payload:        payloadBytes,
	})
	if err != nil {
		return map[string]int{}, err
	}

	var respPayloadMap map[string]interface{}
	err = json.Unmarshal(resp.Payload, &respPayloadMap)
	if err != nil {
		return map[string]int{}, err
	}

	var getCountResponse models.GetCountResponse
	err = json.Unmarshal([]byte(respPayloadMap["body"].(string)), &getCountResponse)
	if err != nil {
		return map[string]int{}, err
	}

	fmt.Println(getCountResponse)

	return getCountResponse.Count, nil
}

func main() {
	lambda.Start(handler)
}
