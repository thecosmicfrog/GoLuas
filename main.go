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

/*
 * readRpaForecast reads in the XML returned from the RPA Luas forecast API and converts it into a struct of type rpaForecastModel.
 * It returns an instance of this rpaForecastModel.
 */
func readRpaForecast(reader io.Reader) (rpaForecastModel, error) {
	var rpaForecast rpaForecastModel

	if err := xml.NewDecoder(reader).Decode(&rpaForecast); err != nil {
		return rpaForecast, err
	}

	return rpaForecast, nil
}

/*
 * readRpaFareCalc reads in the XML returned from the RPA Luas fares API and converts it into a struct of type rpaFareCalcModel.
 * It returns an instance of this rpaFareCalcModel.
 */
func readRpaFareCalc(reader io.Reader) (rpaFareCalcModel, error) {
	var rpaFareCalc rpaFareCalcModel

	if err := xml.NewDecoder(reader).Decode(&rpaFareCalc); err != nil {
		return rpaFareCalc, err
	}

	return rpaFareCalc, nil
}

/*
 * createGoLuasForecast converts an rpaForecastModel object into a goLuasForecastModel object. The GoLuas forecast model is
 * different in structure to that used in the RPA API.
 * It returns a GoLuas forecast.
 */
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

/*
 * getStop performs a simple DynamoDB query, looking for a stop ID and returning the corresponding document with all stop details.
 * It returns a stopModel and any errors collected during the DynamoDB query process.
 */
func getStop(stopID string) (stopModel, error) {
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
		log.Printf("%s %s: %s", logPrefixError, msg, err.Error())
	}

	stop := stopModel{}

	err = dynamodbattribute.UnmarshalMap(result.Item, &stop)

	if err != nil {
		msg := "Failed to unmarshal record"
		log.Printf("%s %s: %s", logPrefixError, msg, err)

		return stopModel{}, err
	}

	return stop, nil
}

/*
 * getStopForecast requests a forecast from the RPA API for a given stop ID and converts it into a GoLuas forecast JSON oject.
 * It returns a JSON object representing a GoLuas forecast model, and any errors encountered during the process.
 */
func getStopForecast(rpaForecastURL, stopID string) ([]byte, error) {
	msg := fmt.Sprintf("Stop forecast requested for param station=%s", stopID)
	log.Printf("%s %s", logPrefixInfo, msg)

	response, err := http.Get(
		fmt.Sprintf(
			"%saction=forecast&stop=%s",
			rpaForecastURL,
			stopID,
		),
	)

	if err != nil {
		msg := "Error establishing HTTP connection to RPA API"
		log.Printf("%s %s: %s", logPrefixError, msg, err)
	} else {
		defer response.Body.Close()

		body, err := ioutil.ReadAll(response.Body)

		if err != nil {
			fmt.Printf("Error reading response body: %s", err)
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

/*
 * getFares requests fares data from the RPA API for a given pair of stops and number of adults and children, and converts it into
 * JSON.
 * It returns a JSON object representing a standard RPA fare calculation, and any errors encountered during the process.
 */
 func getFares(rpaForecastURL, farecalcFrom, farecalcTo, farecalcAdults, farecalcChildren string) ([]byte, error) {
	msg := fmt.Sprintf(
		"Fare calculation requested for params from=%s,to=%s,adults=%s,children=%s",
		farecalcFrom, farecalcTo, farecalcAdults, farecalcChildren,
	)
	log.Printf("%s %s", logPrefixInfo, msg)

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
		log.Printf("%s %s: %s", logPrefixError, msg, err)
	} else {
		defer response.Body.Close()

		body, err := ioutil.ReadAll(response.Body)

		if err != nil {
			msg := "Error reading response body"
			log.Printf("%s %s: %s", logPrefixError, msg, err)
		}

		bodyStr := string(body)
		bodyReader := strings.NewReader(bodyStr)

		rpaFareCalc, err := readRpaFareCalc(bodyReader)

		/* In the case of a fare calculation, we just want the result (RpaFareCalc.Result). */
		rpaFareCalcJSON, err := json.Marshal(&rpaFareCalc.Result)

		if err != nil {
			msg := "Error marshaling RPA fare calculation to JSON"
			log.Printf("%s %s: %s", logPrefixError, msg, err)
		} else {
			return rpaFareCalcJSON, nil
		}
	}

	return nil, err
}

/*
 * createResponse is a simple wrapper function around the events.APIGatewayProxyResponse type. It is called to create a HTTP
 * response from GoLuas and accepts an events.APIGatewayProxyRequest type, as well as a response body and status code.
 * It returns an events.APIGatewayProxyResponse type.
 */
func createResponse(request events.APIGatewayProxyRequest, body string, statusCode int) events.APIGatewayProxyResponse {
	response := events.APIGatewayProxyResponse{
		Body:       body,
		StatusCode: statusCode,
	}

	requestID := request.RequestContext.RequestID
	requestQueryStringParameters := request.QueryStringParameters
	responseStatusCode := response.StatusCode
	responseBody := response.Body

	msg := fmt.Sprintf(
		"Responding to request %s for parameters %s with status code %v and response object: %v",
		requestID, requestQueryStringParameters, responseStatusCode, responseBody,
	)
	log.Printf("%s %s", logPrefixInfo, msg)

	return response
}

/*
 * handleRequest is the primary driver function of GoLuas. It handles a request and routes it to the appropriate function for
 * further processing.
 * It returns an events.APIGatewayProxyResponse type and an error representing any issues collected in downstream functions.
 */
func handleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	requestQueryStringParameters := request.QueryStringParameters

	msg := fmt.Sprintf(
		"Processing request data for request %s with query string parameters %s",
		request.RequestContext.RequestID, requestQueryStringParameters,
	)
	log.Printf("%s %s", logPrefixInfo, msg)

	ver := requestQueryStringParameters["ver"]
	action := requestQueryStringParameters["action"]
	stopID := requestQueryStringParameters["station"]
	from := requestQueryStringParameters["from"]
	to := requestQueryStringParameters["to"]
	adults := requestQueryStringParameters["adults"]
	children := requestQueryStringParameters["children"]

	var rpaForecastURL string

	if ver == "2" {
		rpaForecastURL = rpaForecastURLV2
	} else {
		rpaForecastURL = rpaForecastURLV1
	}

	if action == "times" && stopID != "" {
		stop, err := getStop(stopID)

		if err != nil {
			msg := fmt.Sprintf("Error getting stop for param station=%s", stopID)
			log.Printf("%s %s: %s", logPrefixError, msg, err)

			return createResponse(request, responseMessageGeneralTimesError, 500), err
		}

		if stop.DisplayName == "" {
			msg := fmt.Sprintf("Stop not found for param station=%s", stopID)
			log.Printf("%s %s", logPrefixInfo, msg)

			return createResponse(request, responseMessageUnknownStop, 404), nil
		}

		stopForecast, err := getStopForecast(rpaForecastURL, stopID)
		stopForecastStr := string(stopForecast)

		return createResponse(request, stopForecastStr, 200), nil

	} else if action == "farecalc" && from != "" && to != "" && adults != "" && children != "" {
		fareCalc, err := getFares(rpaForecastURL, from, to, adults, children)
		fareCalcStr := string(fareCalc)

		if err != nil {
			msg := fmt.Sprintf(
				"Error getting fare calculation for params from=%s,to=%s,adults=%s,children=%s",
				from, to, adults, children,
			)
			log.Printf("%s %s: %s", logPrefixError, msg, err)

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
