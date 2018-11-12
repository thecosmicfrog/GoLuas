package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

func readRpaForecast(reader io.Reader) (rpaForecastModel, error) {
	var rpaForecast rpaForecastModel

	if err := xml.NewDecoder(reader).Decode(&rpaForecast); err != nil {
		return rpaForecast, err
	}

	return rpaForecast, nil
}

func readRpaFareCalc(reader io.Reader) (rpaFareCalcModel, error) {
	var rpaFareCalc rpaFareCalcModel

	if err := xml.NewDecoder(reader).Decode(&rpaFareCalc); err != nil {
		return rpaFareCalc, err
	}

	return rpaFareCalc, nil
}

func createGoLuasForecast(rpaForecast rpaForecastModel) goLuasForecastModel {
	forecast := goLuasForecastModel{}
	forecast.Message = rpaForecast.Message
	forecast.Status.Inbound.Message = rpaForecast.Directions[0].StatusMessage
	forecast.Status.Inbound.ForecastsEnabled = rpaForecast.Directions[0].ForecastsEnabled
	forecast.Status.Inbound.OperatingNormally = rpaForecast.Directions[0].OperatingNormally
	forecast.Status.Outbound.Message = rpaForecast.Directions[1].StatusMessage
	forecast.Status.Outbound.ForecastsEnabled = rpaForecast.Directions[1].ForecastsEnabled
	forecast.Status.Outbound.OperatingNormally = rpaForecast.Directions[1].OperatingNormally

	/*
	 * directionIndex 0: Inbound tram.
	 * directionIndex 1: Outbound tram.
	 */
	for directionIndex := 0; directionIndex <= 1; directionIndex++ {
		var direction string

		switch directionIndex {
		case 0:
			direction = "Inbound"
		case 1:
			direction = "Outbound"
		default:
			panic(errors.New("direction is neither Inbound nor Outbound"))
		}

		for tramIndex := range rpaForecast.Directions[directionIndex].Trams {
			if rpaForecast.Directions[directionIndex].Trams[tramIndex].DueMins != "" {
				tram := goLuasTramModel{}
				tram.Direction = direction
				tram.DueMinutes = rpaForecast.Directions[directionIndex].Trams[tramIndex].DueMins
				tram.Destination = rpaForecast.Directions[directionIndex].Trams[tramIndex].Destination

				forecast.Trams = append(forecast.Trams, tram)
			}
		}
	}

	return forecast
}

func getFares(rpaForecastURL, farecalcFrom, farecalcTo, farecalcAdults, farecalcChildren string) ([]byte, error) {
	msg := fmt.Sprintf(
		"Fare calculation requested for params from=%s,to=%s,adults=%s,children=%s",
		farecalcFrom, farecalcTo, farecalcAdults, farecalcChildren,
	)
	log.Printf("%s %s\n", logPrefixInfo, msg)

	response, err := http.Get(
		fmt.Sprintf(
			"%saction=farecalc&from=%s&to=%s&adults=%s&children=%s",
			rpaForecastURL,
			farecalcFrom,
			farecalcTo,
			farecalcAdults,
			farecalcChildren,
		),
	)

	if err != nil {
		msg := "Error establishing HTTP connection RPA API"
		log.Printf("%s %s: %s\n", logPrefixError, msg, err)
	} else {
		defer response.Body.Close()

		body, err := ioutil.ReadAll(response.Body)

		if err != nil {
			msg := "Error reading response body"
			log.Printf("%s %s: %s\n", logPrefixError, msg, err)
		}

		bodyStr := string(body)
		bodyReader := strings.NewReader(bodyStr)

		rpaFareCalc, err := readRpaFareCalc(bodyReader)

		/* In the case of a fare calculation, we just want the result (RpaFareCalc.Result). */
		rpaFareCalcJSON, err := json.Marshal(&rpaFareCalc.Result)

		if err != nil {
			msg := "Error marshaling RPA fare calculation to JSON"
			log.Printf("%s %s: %s\n", logPrefixError, msg, err)
		} else {
			return rpaFareCalcJSON, nil
		}
	}

	return nil, err
}

func getStopForecast(rpaForecastURL, stopID string) ([]byte, error) {
	msg := fmt.Sprintf("Stop forecast requested for param station=%s", stopID)
	log.Printf("%s %s\n", logPrefixInfo, msg)

	response, err := http.Get(
		fmt.Sprintf(
			"%saction=forecast&stop=%s",
			rpaForecastURL,
			stopID,
		),
	)

	if err != nil {
		msg := "Error establishing HTTP connection RPA API"
		log.Printf("%s %s: %s\n", logPrefixError, msg, err)
	} else {
		defer response.Body.Close()

		body, err := ioutil.ReadAll(response.Body)

		if err != nil {
			fmt.Printf("Error reading response body: %s\n", err)
		}

		bodyStr := string(body)
		bodyReader := strings.NewReader(bodyStr)

		rpaForecast, err := readRpaForecast(bodyReader)

		goLuasForecast := createGoLuasForecast(rpaForecast)
		goLuasForecastJSON, err := json.Marshal(&goLuasForecast)

		return goLuasForecastJSON, nil
	}

	return nil, err
}

func getStop(rpaForecastURL, stopID string) (stopModel, error) {
	awsSession, err := session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	})

	dynamoDBService := dynamodb.New(awsSession)

	result, err := dynamoDBService.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(dynamoDBTable),
		Key: map[string]*dynamodb.AttributeValue{
			"shortName": {
				S: aws.String(stopID),
			},
		},
	})

	if err != nil {
		msg := fmt.Sprintf("Error getting item from DynamoDB table with shortName:%s", stopID)
		log.Printf("%s %s: %s\n", logPrefixError, msg, err.Error())
	}

	stop := stopModel{}

	err = dynamodbattribute.UnmarshalMap(result.Item, &stop)

	if err != nil {
		msg := "Failed to unmarshal record"
		log.Printf("%s %s: %s\n", logPrefixError, msg, err)

		return stopModel{}, err
	}

	return stop, nil
}

func createResponse(request events.APIGatewayProxyRequest, body string, statusCode int) events.APIGatewayProxyResponse {
	response := events.APIGatewayProxyResponse{
		Body:       body,
		StatusCode: statusCode,
	}

	msg := fmt.Sprintf(
		"Responding to requestor %s with response object: %v",
		request.RequestContext.RequestID, response.Body,
	)
	log.Printf("%s %s\n", logPrefixInfo, msg)

	return response
}

func handleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	queryStringParameters := request.QueryStringParameters

	log.Printf("Processing request data for request %s.\n", request.RequestContext.RequestID)
	log.Printf("Query string parameters: %s\n", queryStringParameters)

	var rpaForecastURL string

	ver := queryStringParameters["ver"]
	action := queryStringParameters["action"]
	stopID := queryStringParameters["station"]
	from := queryStringParameters["from"]
	to := queryStringParameters["to"]
	adults := queryStringParameters["adults"]
	children := queryStringParameters["children"]

	if ver == "2" {
		rpaForecastURL = rpaForecastURLV2
	} else {
		rpaForecastURL = rpaForecastURLV1
	}

	if action == "times" && stopID != "" {
		stop, err := getStop(rpaForecastURL, stopID)

		if err != nil {
			msg := fmt.Sprintf("Error getting stop for param station=%s", stopID)
			log.Printf("%s %s: %s\n", logPrefixError, msg, err)

			return createResponse(request, responseMessageGeneralTimesError, 500), err
		}

		if stop.DisplayName == "" {
			msg := fmt.Sprintf("Stop not found for param station=%s", stopID)
			log.Printf("%s %s\n", logPrefixInfo, msg)

			return createResponse(request, responseMessageUnknownStop, 404), nil
		}

		stopForecast, err := getStopForecast(rpaForecastURL, stopID)
		stopForecastStr := string(stopForecast)

		return events.APIGatewayProxyResponse{Body: stopForecastStr, StatusCode: 200}, nil

	} else if action == "farecalc" && from != "" && to != "" && adults != "" && children != "" {
		fareCalc, err := getFares(rpaForecastURL, from, to, adults, children)
		fareCalcStr := string(fareCalc)

		if err != nil {
			msg := fmt.Sprintf(
				"Error getting fare calculation for params from=%s,to=%s,adults=%s,children=%s",
				from, to, adults, children,
			)
			log.Printf("%s %s: %s\n", logPrefixError, msg, err)

			return createResponse(request, responseMessageGeneralFaresError, 500), err
		}

		return createResponse(request, fareCalcStr, 200), nil

	} else if action == "brewcoffee" { /* Why not? */
		return createResponse(request, responseMessageImATeapot, 418), nil
	}

	return createResponse(request, responseMessageInvalidRequest, 400), nil
}

func main() {
	lambda.Start(handleRequest)
}
