package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

type TokenData struct {
	Symbol       string  `json:"symbol"`
	FloorPrice   float64 `json:"floorPrice"`
	ListedCount  int     `json:"listedCount"`
	AvgPrice24Hr float64 `json:"avgPrice24hr"`
	VolumeAll    float64 `json:"volumeAll"`
}

type ExchangeRateResponse struct {
	Success   bool               `json:"success"`
	Terms     string             `json:"terms"`
	Privacy   string             `json:"privacy"`
	Timestamp int64              `json:"timestamp"`
	Date      string             `json:"date"`
	Base      string             `json:"base"`
	Rates     map[string]float64 `json:"rates"`
}

var fxRatesApiKey string

func getCurrencyRates(baseCurrency string, targetCurrencies []string) map[string]float64 {

	var requestCurrencies string
	for i, currency := range targetCurrencies {
		if i == len(targetCurrencies)-1 {
			requestCurrencies += currency
		} else {
			requestCurrencies += currency + ","
		}
	}

	url := fmt.Sprintf(
		"https://api.fxratesapi.com/latest?api_key=%s&base=%s&currencies=%s&resolution=1m&amount=1&places=6&format=json",
		fxRatesApiKey,
		baseCurrency,
		requestCurrencies,
	)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("currency api request error: %s", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("currency api response error: %s", err)
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)
	var exchangeRateResponse ExchangeRateResponse
	err = json.Unmarshal(body, &exchangeRateResponse)
	if err != nil {
		fmt.Printf("Unmarshal error: %s", err)
	}

	return exchangeRateResponse.Rates
}

func getTokenInfo(tokenSymbol string) TokenData {
	url := fmt.Sprintf("https://api-mainnet.magiceden.dev/v2/collections/%s/stats", tokenSymbol)

	req, _ := http.NewRequest("GET", url, nil)

	req.Header.Add("accept", "application/json")

	res, _ := http.DefaultClient.Do(req)

	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)
	var tokenData TokenData
	err := json.Unmarshal(body, &tokenData)
	if err != nil {
		fmt.Printf("Err on json urmarshal: %s", err)
		return TokenData{}
	}

	tokenData.FloorPrice = tokenData.FloorPrice / 1000000000

	return tokenData
}


func main() {

	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading env vars: %s\n", err)
	}

	fxRatesApiKey = os.Getenv("FX_RATES_API_KEY")

	handler := func(w http.ResponseWriter, req *http.Request) {

		tokenSymbols := []string{"tomorrowland_winter", "tomorrowland_love_unity", "the_reflection_of_love"}
		allTokenData := []TokenData{}
		var totalPriceSol float64
		for _, tokenSymbol := range tokenSymbols {
			tokenData := getTokenInfo(tokenSymbol)
			totalPriceSol += tokenData.FloorPrice
			allTokenData = append(allTokenData, tokenData)
		}
		baseCurrency := "SOL"
		targetCurrencies := []string{"USD", "EUR", "GBP"}
		currencyRates := getCurrencyRates(baseCurrency, targetCurrencies)

		tmpl, err := template.ParseFiles("templates/index.html")
		if err != nil {
			fmt.Println(err)
		}
		data := struct {
			AllTokenData  []TokenData
			TotalPriceSol float64
			CurrencyRates map[string]float64
		}{
			AllTokenData:  allTokenData,
			TotalPriceSol: totalPriceSol,
			CurrencyRates: currencyRates,
		}

		err = tmpl.Execute(w, data)
		if err != nil {
			fmt.Println(err)
		}
	}

	http.HandleFunc("/", handler)

	addr := "localhost:42069"
	log.Printf("listening to %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
