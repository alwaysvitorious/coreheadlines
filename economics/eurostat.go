package economics

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
)

func GetEurostat() []EconomicsResponse {
	var results []EconomicsResponse

	for _, dataset := range EurostatDatasets {
		url := fmt.Sprintf(
			"https://ec.europa.eu/eurostat/api/dissemination/statistics/1.0/data/%s?FORMAT=JSON&lang=EN%s&geo=EU27_2020&lastTimePeriod=4",
			dataset.Key,
			dataset.Param,
		)

		resp, err := http.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		var euroResp EurostatResponse
		if err := json.NewDecoder(resp.Body).Decode(&euroResp); err != nil {
			continue
		}

		// reverse‑map: time‑dimension position -> raw date string
		indexToDate := make(map[int]string, len(euroResp.Dimension.Time.Category.Index))
		for raw, pos := range euroResp.Dimension.Time.Category.Index {
			indexToDate[pos] = raw
		}
		timeCount := len(indexToDate)

		// find latest non‑NaN flat index & its value
		latestFlatIdx := -1
		var latestVal float64
		for key, v := range euroResp.Value {
			if math.IsNaN(v) {
				continue
			}
			idx, err := strconv.Atoi(key)
			if err != nil {
				continue
			}
			if idx > latestFlatIdx {
				latestFlatIdx = idx
				latestVal = v
			}
		}

		if latestFlatIdx < 0 {
			// no data at any time
			continue
		}

		// peel off time‑dimension index
		timeIdx := latestFlatIdx % timeCount // e.g. 16 % 6 == 4
		rawDate := indexToDate[timeIdx]      // e.g. indexToDate[4] == "2025-05"

		dp := DataPoint{
			Index: dataset.Index,
			Name:  dataset.Name,
			Date:  formatDate(rawDate),
			Value: latestVal,
		}
		results = append(results, EconomicsResponse{
			DataPoints: []DataPoint{dp},
		})

	}

	if len(results) == 0 {
		return []EconomicsResponse{}
	}

	return results
}
