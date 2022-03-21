package main

import (
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
)

type KanyeResponseResult struct {
	Quote string `json:"quote"`
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

func main() {

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
}
