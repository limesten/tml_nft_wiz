package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
)

type TokenData struct {
	Symbol      string  `json:"symbol"`
    FloorPrice  int64   `json:"floorPrice"`
    ListedCount int     `json:"listedCount"`
    AvgPrice24Hr float64   `json:"avgPrice24hr"`
    VolumeAll   float64 `json:"volumeAll"`
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
	return tokenData
}

func main() {

	handler := func(w http.ResponseWriter, req *http.Request) {

		tokenSymbols := []string{"tomorrowland_winter", "tomorrowland_love_unity", "the_reflection_of_love"}
		allTokenData := []TokenData{}
		for _, tokenSymbol := range tokenSymbols {
			tokenData := getTokenInfo(tokenSymbol)
			allTokenData = append(allTokenData, tokenData)
		}

		tmpl, err := template.ParseFiles("templates/index.html")
		if err != nil {
			fmt.Println(err)
		}
		err = tmpl.Execute(w, allTokenData)
		if err != nil {
			fmt.Println(err)
		}
	}

	http.HandleFunc("/", handler)

	addr := "localhost:42069"
	log.Printf("listening to %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
