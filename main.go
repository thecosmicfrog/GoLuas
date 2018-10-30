package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type GoLuasForecast struct {
	Message string       `json:"message"`
	Status  GoLuasStatus `json:"status"`
	Trams   []GoLuasTram `json:"trams"`
}

type GoLuasStatus struct {
	Inbound  GoLuasStatusDirection `json:"inbound"`
	Outbound GoLuasStatusDirection `json:"outbound"`
}

type GoLuasStatusDirection struct {
	Message           string `json:"message"`
	ForecastsEnabled  string `json:"forecastsEnabled"`
	OperatingNormally string `json:"operatingNormally"`
}

type GoLuasTram struct {
	Direction   string `json:"direction"`
	DueMinutes  string `json:"dueMinutes"`
	Destination string `json:"destination"`
}

type RpaForecast struct {
	Created    string      `xml:"created,attr" json:"created"`
	Stop       string      `xml:"stop,attr" json:"stop"`
	StopAbv    string      `xml:"stopAbv,attr" json:"stopAbv"`
	Message    string      `xml:"message" json:"message"`
	Directions []Direction `xml:"direction" json:"direction"`
}

type Direction struct {
	Name              string `xml:"name,attr" json:"name"`
	StatusMessage     string `xml:"statusMessage,attr" json:"statusMessage"`
	ForecastsEnabled  string `xml:"forecastsEnabled,attr" json:"forecastsEnabled"`
	OperatingNormally string `xml:"operatingNormally,attr" json:"operatingNormally"`
	Trams             []Tram `xml:"tram" json:"tram"`
}

type Tram struct {
	DueMins     string `xml:"dueMins,attr" json:"dueMins"`
	Destination string `xml:"destination,attr" json:"destination"`
}

func ReadRpaForecast(reader io.Reader) (RpaForecast, error) {
	var rpaForecast RpaForecast

	if err := xml.NewDecoder(reader).Decode(&rpaForecast); err != nil {
		return rpaForecast, err
	}

	return rpaForecast, nil
}

func handleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Processing request data for request %s.\n", request.RequestContext.RequestID)
	log.Printf("Body size = %d.\n", len(request.Body))

	log.Println("Headers:")
	for key, value := range request.Headers {
		log.Printf("    %s: %s\n", key, value)
	}

	log.Println(request.PathParameters)

	queryStringParameters := request.QueryStringParameters["stopid"]

	response, err := http.Get("http://luasforecasts.rpa.ie/xml/get.ashx?encrypt=false&action=forecast&stop=" + queryStringParameters + "&ver=2")

	if err != nil {
		fmt.Printf("%s", err)
	} else {
		defer response.Body.Close()

		body, err := ioutil.ReadAll(response.Body)

		if err != nil {
			fmt.Printf("%s", err)
		}

		bodyStr := string(body)
		bodyReader := strings.NewReader(bodyStr)

		rpaForecast, err := ReadRpaForecast(bodyReader)

		goLuasForecast := GoLuasForecast{}
		goLuasForecast.Message = rpaForecast.Message
		goLuasForecast.Status.Inbound.Message = rpaForecast.Directions[0].StatusMessage
		goLuasForecast.Status.Inbound.ForecastsEnabled = rpaForecast.Directions[0].ForecastsEnabled
		goLuasForecast.Status.Inbound.OperatingNormally = rpaForecast.Directions[0].OperatingNormally
		goLuasForecast.Status.Outbound.Message = rpaForecast.Directions[1].StatusMessage
		goLuasForecast.Status.Outbound.ForecastsEnabled = rpaForecast.Directions[1].ForecastsEnabled
		goLuasForecast.Status.Outbound.OperatingNormally = rpaForecast.Directions[1].OperatingNormally

		for i := range rpaForecast.Directions[0].Trams {
			goLuasTramInbound := GoLuasTram{}

			goLuasTramInbound.Direction = "Inbound"

			dueMinsInboundStr := rpaForecast.Directions[0].Trams[i].DueMins
			dueMinsInboundInt, err := strconv.Atoi(rpaForecast.Directions[0].Trams[i].DueMins)
			destinationInboundStr := rpaForecast.Directions[0].Trams[i].Destination

			if err != nil {
				// TODO: error
			} else {
				if dueMinsInboundInt > 0 || dueMinsInboundStr == "DUE" {
					goLuasTramInbound.DueMinutes = dueMinsInboundStr
					goLuasTramInbound.Destination = destinationInboundStr

					goLuasForecast.Trams = append(goLuasForecast.Trams, goLuasTramInbound)
				}
			}
		}

		for i := range rpaForecast.Directions[1].Trams {
			goLuasTramOutbound := GoLuasTram{}

			goLuasTramOutbound.Direction = "Outbound"

			dueMinsOutboundStr := rpaForecast.Directions[1].Trams[i].DueMins
			dueMinsOutboundInt, err := strconv.Atoi(rpaForecast.Directions[1].Trams[i].DueMins)
			destinationOutboundStr := rpaForecast.Directions[1].Trams[i].Destination

			if err != nil {
				// TODO: error
			} else {
				if dueMinsOutboundInt > 0 || dueMinsOutboundStr == "DUE" {
					goLuasTramOutbound.DueMinutes = dueMinsOutboundStr
					goLuasTramOutbound.Destination = destinationOutboundStr

					goLuasForecast.Trams = append(goLuasForecast.Trams, goLuasTramOutbound)
				}
			}
		}

		goLuasForecastJson, err := json.Marshal(&goLuasForecast)

		return events.APIGatewayProxyResponse{Body: string(goLuasForecastJson), StatusCode: 200}, nil
	}

	return events.APIGatewayProxyResponse{}, nil
}

func main() {
	/* Start Testing */
	// x := events.APIGatewayProxyRequest{}
	/* End Testing */

	lambda.Start(handleRequest)
}
