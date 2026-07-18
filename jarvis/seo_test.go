package main

import (
	"reflect"
	"testing"
)

func TestParseGSCRows(t *testing.T) {
	t.Parallel()

	input := []byte(`{
		"rows": [
			{"keys":["autonomous ai agent"],"clicks":12,"impressions":240,"ctr":0.05,"position":7.25},
			{"keys":["https://noxioai.com/en/ai-employees"],"clicks":3,"impressions":300,"ctr":0.01,"position":11.5}
		],
		"responseAggregationType": "byProperty"
	}`)

	got, err := parseGSCRows(input)
	if err != nil {
		t.Fatalf("parseGSCRows() error = %v", err)
	}
	want := []gscRow{
		{Keys: []string{"autonomous ai agent"}, Clicks: 12, Impressions: 240, CTR: 0.05, Position: 7.25},
		{Keys: []string{"https://noxioai.com/en/ai-employees"}, Clicks: 3, Impressions: 300, CTR: 0.01, Position: 11.5},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseGSCRows() = %#v, want %#v", got, want)
	}
}

func TestFilterSEOOpportunities(t *testing.T) {
	t.Parallel()

	queryRows := []gscRow{
		{Keys: []string{"already top"}, Impressions: 500, Position: 4.9},
		{Keys: []string{"position five"}, Impressions: 250, Position: 5},
		{Keys: []string{"middle opportunity"}, Impressions: 900, Position: 10},
		{Keys: []string{"position fifteen"}, Impressions: 100, Position: 15},
		{Keys: []string{"too low"}, Impressions: 800, Position: 15.1},
	}
	pageRows := []gscRow{
		{Keys: []string{"/strong-page"}, Impressions: 800, CTR: 0.019},
		{Keys: []string{"/threshold-page"}, Impressions: 100, CTR: 0.01},
		{Keys: []string{"/not-enough-impressions"}, Impressions: 99, CTR: 0.001},
		{Keys: []string{"/ctr-not-low"}, Impressions: 700, CTR: 0.02},
	}

	got := filterSEOOpportunities(queryRows, pageRows)

	wantQueries := []string{"middle opportunity", "position five", "position fifteen"}
	if names := rowKeys(got.OnePushQueries); !reflect.DeepEqual(names, wantQueries) {
		t.Errorf("one-push queries = %v, want %v", names, wantQueries)
	}
	wantPages := []string{"/strong-page", "/threshold-page"}
	if names := rowKeys(got.LowCTRPages); !reflect.DeepEqual(names, wantPages) {
		t.Errorf("low-CTR pages = %v, want %v", names, wantPages)
	}
}

func rowKeys(rows []gscRow) []string {
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		if len(row.Keys) > 0 {
			keys = append(keys, row.Keys[0])
		}
	}
	return keys
}
