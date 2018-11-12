package main

const awsRegion string = "eu-west-1"
const dynamoDBTable string = "GoLuasStops"
const logPrefixError string = "ERROR"
const logPrefixInfo string = "INFO"
const responseMessageInvalidRequest string = `{"message": "Invalid request"}`
const responseMessageGeneralFaresError string = `{"message": "General error getting fares"}`
const responseMessageGeneralTimesError string = `{"message": "General error getting times"}`
const responseMessageImATeapot string = `{"message": "Your request to brew coffee with this server has failed. This HTCPCP server is a teapot. The requested entity body is short and stout. Tip me over and pour me out."}`
const responseMessageUnknownStop string = `{"message": "Unknown stop"}`
const rpaForecastURLV1 string = "http://luasforecasts.rpa.ie/xml/get.ashx?encrypt=false&ver=1&"
const rpaForecastURLV2 string = "http://luasforecasts.rpa.ie/xml/get.ashx?encrypt=false&ver=2&"
