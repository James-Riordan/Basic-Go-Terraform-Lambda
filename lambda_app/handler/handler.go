// Logic for application

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
)

type Response events.APIGatewayProxyResponse

// Handler - interface
type Handler interface {
	Run(ctx context.Context, event events.APIGatewayCustomAuthorizerRequest) (Response, error)
}

type lambdaHander struct {
	lambdaName string
}

type LambdaResponse struct {
	Message string
}

type KanyeResponseResult struct {
	Quote string `json:"quote"`
}

type Payload struct {
	Quote KanyeResponseResult
	IP 
}

func shuffleStringArray(src []string) []string {
	final := make([]string, len(src))
	rand.Seed(time.Now().UTC().UnixNano())
	perm := rand.Perm(len(src))

	for i, v := range perm {
		final[v] = src[i]
	}
	return final
}

func (l lambdaHander) Run(ctx context.Context, event events.APIGatewayCustomAuthorizerRequest) (Response, error) {

	kanyeAPICalls := 3
	var kanyeMumboJumbo []string
	var kanyeResult KanyeResponseResult

	for i := 0; i < kanyeAPICalls; i++ {
		kanyeAPICallResponse, err := http.Get("https://api.kanye.rest/")
		if err != nil {
			log.Fatalln(err)
		}
		contents, err := ioutil.ReadAll(kanyeAPICallResponse.Body)
		json.Unmarshal(contents, &kanyeResult)
		kanyeQuote := kanyeResult.Quote
		fmt.Printf("Kanye: %v\n", kanyeQuote)
		quoteSplitUp := strings.Fields(kanyeQuote)
		for _, v := range quoteSplitUp {
			fmt.Println(v)
			reg, err := regexp.Compile("[^a-zA-Z0-9]+")
			if err != nil {
				log.Fatal(err)
			}
			processedString := reg.ReplaceAllString(v, "")
			kanyeMumboJumbo = append(kanyeMumboJumbo, processedString)
		}
	}

	quoteLength := math.Round((float64(len(kanyeMumboJumbo))) / float64(3))

	quoteToReturn := ""

	kanyeMumboJumbo = shuffleStringArray(kanyeMumboJumbo)

	for i := 0; i < int(quoteLength); i++ {
		quoteToReturn = quoteToReturn + " " + kanyeMumboJumbo[i]
	}

	fmt.Printf(quoteToReturn)

	payload := {quoteToRet}

	response, err := json.Marshal(quoteToReturn)

	res := Response{
		StatusCode:      200,
		IsBase64Encoded: false,
		Headers: map[string]string{
			"Access-Control-Allow-Origin":      "*",
			"Access-Control-Allow-Credentials": "true",
			"Cache-Control":                    "no-cache; no-store",
			"Content-Type":                     "application/json",
			"Content-Security-Policy":          "default-src self",
			"Strict-Transport-Security":        "max-age=31536000; includeSubDomains",
			"X-Content-Type-Options":           "nosniff",
			"X-XSS-Protection":                 "1; mode=block",
			"X-Frame-Options":                  "DENY",
		},
		Body: string(response),
	}

	return res, err
}

// NewLambdaHandler -
func NewLambdaHandler(
	lambdaName string,
) *lambdaHander {
	return &lambdaHander{
		lambdaName: lambdaName,
	}
}
