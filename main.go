package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv")

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

type apiConfig struct {
	CurrencyRates  map[string]float64
	Prices         map[string]float64
	Tokens         map[string]TokenData
	RatesUpdatedAt time.Time
	TotalPriceSol  float64
	fxRatesApiKey  string
}

var fxRatesApiKey string

func (cfg *apiConfig) getCurrencyRates() {
	baseCurrency := "SOL"
	targetCurrencies := []string{"USD", "EUR", "GBP", "SEK"}

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

	prices := make(map[string]float64)
	for currency, rate := range exchangeRateResponse.Rates {
		prices[currency] = rate * cfg.TotalPriceSol
	}
	cfg.CurrencyRates = exchangeRateResponse.Rates
	cfg.Prices = prices
}

func (cfg *apiConfig) getTokenData() {
	tokenSymbols := []string{"tomorrowland_winter", "tomorrowland_love_unity", "the_reflection_of_love"}
	var totalPriceSol float64

	if cfg.Tokens == nil {
		cfg.Tokens = make(map[string]TokenData)
	}

	for _, tokenSymbol := range tokenSymbols {
		url := fmt.Sprintf("https://api-mainnet.magiceden.dev/v2/collections/%s/stats", tokenSymbol)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Add("accept", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Printf("Api req err: %s", err)
			return
		}
		defer res.Body.Close()

		body, _ := io.ReadAll(res.Body)
		var tokenData TokenData
		err = json.Unmarshal(body, &tokenData)
		if err != nil {
			fmt.Printf("Err on json urmarshal: %s", err)
			return
		}

		tokenData.FloorPrice = tokenData.FloorPrice / 1000000000
		totalPriceSol += tokenData.FloorPrice

		cfg.Tokens[tokenData.Symbol] = tokenData
	}
	cfg.TotalPriceSol = totalPriceSol
}

func getTotalPricePerCurrency(currencyRates map[string]float64, totalPriceSol float64) map[string]float64 {
	prices := make(map[string]float64)
	for currency, rate := range currencyRates {
		prices[currency] = rate * totalPriceSol
	}
	return prices
}

func (cfg *apiConfig) handlerGetData(w http.ResponseWriter, req *http.Request) {
	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil {
		fmt.Printf("HTML template parsing error: %s", err)
		return
	}

	data := struct {
		CurrencyRates  map[string]float64
		Tokens         map[string]TokenData
		Prices         map[string]float64
		RatesUpdatedAt time.Time
		TotalPriceSol  float64
	}{
		CurrencyRates:  cfg.CurrencyRates,
		Prices:         cfg.Prices,
		Tokens:         cfg.Tokens,
		RatesUpdatedAt: cfg.RatesUpdatedAt,
		TotalPriceSol:  cfg.TotalPriceSol,
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		fmt.Println(err)
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading env vars: %s\n", err)
	}
	apiCfg := apiConfig{}
	fxRatesApiKey = os.Getenv("FX_RATES_API_KEY")
	apiCfg.fxRatesApiKey = fxRatesApiKey
	apiCfg.getTokenData()
	apiCfg.getCurrencyRates()

	http.HandleFunc("/", apiCfg.handlerGetData)

	// Refresh token data and currency rates based on updateFrequency
	updateFrequency := 5 * time.Minute
	ticker := time.NewTicker(updateFrequency)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <- ticker.C:
			apiCfg.getTokenData()
			apiCfg.getCurrencyRates()
			case <- quit:
			ticker.Stop()
			return
			}
		}
	}()

	addr := "localhost:42069"
	log.Printf("listening to %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
