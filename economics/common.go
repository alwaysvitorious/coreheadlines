package economics

import "time"

type DataPoint struct {
	Index int     `json:"index"`
	Name  string  `json:"name"`
	Date  string  `json:"date"`
	Value float64 `json:"value"`
}

type EconomicsResponse struct {
	DataPoints []DataPoint `json:"data_points"`
}

type EurostatResponse struct {
	Dimension struct {
		Time struct {
			Category struct {
				Index map[string]int `json:"index"`
			} `json:"category"`
		} `json:"time"`
	} `json:"dimension"`
	Value map[string]float64 `json:"value"`
}

type FREDResponse struct {
	Observations []struct {
		Date  string `json:"date"`
		Value string `json:"value"`
	} `json:"observations"`
}

type Dataset struct {
	Index int     `json:"-"`
	Name  string  `json:"name"`
	Param string  `json:"param,omitempty"`
	Path  string  `json:"path,omitempty"`
	Key   string  `json:"key,omitempty"`
	Units float64 `json:"units,omitempty"`
}

var EurostatDatasets = []Dataset{
	{Index: 2, Key: "une_rt_m", Param: "&unit=PC_ACT&age=TOTAL&sex=T&s_adj=SA", Name: "EU unemploy."},
	{Index: 4, Key: "prc_hicp_manr", Param: "&coicop=CP00", Name: "EU inflation"},
}

var FREDDatasets = []Dataset{
	{Index: 1, Key: "UNRATE", Name: "US unemploy."},
	{Index: 3, Key: "CORESTICKM159SFRBATL", Name: "US inflation"},
	{Index: 5, Key: "DFEDTARL", Name: "US Fedfunds obj."},
	{Index: 6, Key: "FEDFUNDS", Name: "US Fedfunds eff."},
	{Index: 7, Key: "ECBMRRFR", Name: "EU MRO obj."},
	{Index: 8, Key: "ECBESTRVOLWGTTRMDMNRT", Name: "EU €STR eff."},
}

func formatDate(raw string) string {
	layouts := []string{"2006-01-02", "2006-01"}
	for _, layout := range layouts {
		t, err := time.Parse(layout, raw)
		if err == nil {
			return t.Format("Jan 2, 2006")
		}
	}
	return raw // fallback
}
