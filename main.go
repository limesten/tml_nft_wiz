package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type TokenData struct {
	Symbol       string  `json:"symbol"`
	FloorPrice   float64 `json:"floorPrice"`
	ListedCount  int     `json:"listedCount"`
	AvgPrice24Hr float64 `json:"avgPrice24hr"`
	VolumeAll    float64 `json:"volumeAll"`
	FiatPrices   map[string]float64
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

type TokenDataHistory struct {
	ID         int64
	Token      string
	Timestamp  string
	FloorPrice float64
}

type CombinedPriceHistoryDate struct {
	Date string
	SOL  float64
	EUR  float64
	USD  float64
	GBP  float64
	SEK  float64
}

type CombinedPriceHistory struct {
	Dates      []string
	Currencies map[string][]float64
}

type apiConfig struct {
	CurrencyRates  map[string]float64
	Prices         map[string]float64
	Tokens         map[string]TokenData
	RatesUpdatedAt string
	TotalPriceSol  float64
	fxRatesApiKey  string
	DB             *sql.DB
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
		price := rate * cfg.TotalPriceSol
		prices[currency] = math.Round(price)

		utcTime := time.Now().UTC()
		formattedUtcTimeStamp := utcTime.Format("2006-01-02 15:04:05")
		formattedUtcDate := utcTime.Format("2006-01-02")

		sqlStmt := `
            insert or replace into exchange_rates (id, currency, timestamp, sol_exchange_rate) values
            ((select id from exchange_rates where currency = ? and date(timestamp) = ?), ?, ?, ?);
        `
		_, err = cfg.DB.Exec(sqlStmt, currency, formattedUtcDate, currency, formattedUtcTimeStamp, rate)
		if err != nil {
			log.Printf("Failed to update rates for %s, error: %v", currency, err)
		}
	}

	for token, tokenData := range cfg.Tokens {
		tokenDataTemp := tokenData
		tempFiatPrices := make(map[string]float64)
		for currency, rate := range exchangeRateResponse.Rates {
			tempFiatPrices[currency] = tokenData.FloorPrice * rate
			tokenDataTemp.FiatPrices = tempFiatPrices
		}
		cfg.Tokens[token] = tokenDataTemp
	}

	timeStamp := time.Unix(exchangeRateResponse.Timestamp, 0)
	cfg.RatesUpdatedAt = timeStamp.Format(time.RFC822)
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

		solFloorPrice := tokenData.FloorPrice / 1000000000

		tokenData.FloorPrice = math.Floor(solFloorPrice*100) / 100

		totalPriceSol += tokenData.FloorPrice

		cfg.Tokens[tokenData.Symbol] = tokenData

		utcTime := time.Now().UTC()
		formattedUtcTimeStamp := utcTime.Format("2006-01-02 15:04:05")
		formattedUtcDate := utcTime.Format("2006-01-02")

		sqlStmt := `
            insert or replace into sol_rates (id, token, timestamp, sol) values
            ((select id from sol_rates where token = ? and date(timestamp) = ?), ?, ?, ?);
        `
		_, err = cfg.DB.Exec(
			sqlStmt,
			tokenData.Symbol,
			formattedUtcDate,
			tokenData.Symbol,
			formattedUtcTimeStamp,
			tokenData.FloorPrice,
		)
		if err != nil {
			log.Printf("Failed to update rates for %s, error: %v", tokenData.Symbol, err)
		}
	}
	cfg.TotalPriceSol = math.Round(totalPriceSol*100) / 100
}

func (cfg *apiConfig) handlerGetData(w http.ResponseWriter, req *http.Request) {

	takerFee := 1.025

	adjustedPrices := make(map[string]float64)
	for currency, price := range cfg.Prices {
		adjustedPrices[currency] = math.Round((price*takerFee)*1000) / 1000
	}

	adjustedTokens := make(map[string]TokenData)
	for token, tokenData := range cfg.Tokens {
		adjustedToken := tokenData
		adjustedToken.FloorPrice = tokenData.FloorPrice * takerFee
		adjustedToken.FloorPrice = math.Round(adjustedToken.FloorPrice*1000) / 1000
		adjustedTokens[token] = adjustedToken
	}

	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil {
		fmt.Printf("HTML template parsing error: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Prepared individual token history
	/*
		sqlStmt := "select id, token, timestamp, sol from sol_rates"
		rows, err := cfg.DB.Query(sqlStmt)
		if err != nil {
			log.Printf("Failed to fetch sol price history from db. Err: %v\n", err)
		} else {
			for rows.Next() {
				var token TokenDataHistory
				err = rows.Scan(&token.ID, &token.Token, &token.Timestamp, &token.FloorPrice)
				if err != nil {
					log.Printf("Failed to parse token data history from db. Err: %v\n", err)
				} else {
					date := token.Timestamp[:10]

					dates := priceHistory[token.Token].Dates
					floorPrices := priceHistory[token.Token].FloorPrices

					dates = append(dates, date)
					floorPrices = append(floorPrices, token.FloorPrice)

					data := TokenDataHistoryAll{
						Dates:       dates,
						FloorPrices: floorPrices,
					}
					priceHistory[token.Token] = data
				}
			}
		}
	*/

	var combinedPriceHistory CombinedPriceHistory

	sqlStmt := "select date, sol, eur, usd, gbp, sek from v_combined_price_per_date"
	rows, err := cfg.DB.Query(sqlStmt)
	if err != nil {
		log.Printf("Failed to fetch sol price history from db. Err: %v\n", err)
	} else {
		var row CombinedPriceHistoryDate
		dates := []string{}
		currencies := make(map[string][]float64)

		for rows.Next() {
			err = rows.Scan(&row.Date, &row.SOL, &row.EUR, &row.USD, &row.GBP, &row.SEK)
			if err != nil {
				log.Printf("Failed to parse token data history from db. Err: %v\n", err)
			} else {
				dates = append(dates, row.Date)
				currencies["SOL"] = append(currencies["SOL"], row.SOL)
				currencies["EUR"] = append(currencies["EUR"], row.EUR)
				currencies["USD"] = append(currencies["USD"], row.USD)
				currencies["GBP"] = append(currencies["GBP"], row.GBP)
				currencies["SEK"] = append(currencies["SEK"], row.SEK)
			}
		}

		combinedPriceHistory.Dates = dates
		combinedPriceHistory.Currencies = currencies
	}

	combinedPriceHistoryJSON, err := json.Marshal(combinedPriceHistory)
	if err != nil {
		fmt.Println(err)
	}

	data := struct {
		CurrencyRates            map[string]float64
		Tokens                   map[string]TokenData
		Prices                   map[string]float64
		RatesUpdatedAt           string
		TotalPriceSol            float64
		CombinedPriceHistoryJSON string
	}{
		CurrencyRates:            cfg.CurrencyRates,
		Prices:                   adjustedPrices,
		Tokens:                   adjustedTokens,
		RatesUpdatedAt:           cfg.RatesUpdatedAt,
		TotalPriceSol:            cfg.TotalPriceSol,
		CombinedPriceHistoryJSON: string(combinedPriceHistoryJSON),
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func main() {
	apiCfg := apiConfig{}

	db, err := sql.Open("sqlite3", "./tmlwiz.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	apiCfg.DB = db

	fxRatesApiKey = os.Getenv("FX_RATES_API_KEY")
	apiCfg.fxRatesApiKey = fxRatesApiKey
	apiCfg.getTokenData()
	apiCfg.getCurrencyRates()

	fs := http.FileServer(http.Dir("./static/"))
	http.HandleFunc("/", apiCfg.handlerGetData)
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Refresh token data and currency rates based on updateFrequency
	updateFrequency := 10 * time.Minute
	ticker := time.NewTicker(updateFrequency)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				apiCfg.getTokenData()
				apiCfg.getCurrencyRates()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	port := "8080"
	log.Printf("listening on %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))

}
