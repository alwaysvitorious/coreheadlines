package economics

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
)

func GetFED() []EconomicsResponse {
	API_KEY := os.Getenv("FRED_API_KEY")
	var results []EconomicsResponse

	for _, dataset := range FREDDatasets {
		url := fmt.Sprintf("https://api.stlouisfed.org/fred/series/observations?series_id=%s&api_key=%s&file_type=json&sort_order=desc&limit=1",
			dataset.Key, API_KEY)

		resp, err := http.Get(url)
		if err != nil {
			return []EconomicsResponse{}
		}
		defer resp.Body.Close()

		var fredResp FREDResponse
		if err := json.NewDecoder(resp.Body).Decode(&fredResp); err != nil {
			continue
		}

		for _, obs := range fredResp.Observations {
			if obs.Value == "" || obs.Value == "." {
				continue
			}

			value, err := strconv.ParseFloat(obs.Value, 64)
			if err != nil || math.IsNaN(value) {
				continue
			}

			dataPoint := DataPoint{
				Index: dataset.Index,
				Name:  dataset.Name,
				Date:  formatDate(obs.Date),
				Value: value,
			}

			results = append(results, EconomicsResponse{
				DataPoints: []DataPoint{dataPoint},
			})

			break // just latest
		}
	}

	return results
}
