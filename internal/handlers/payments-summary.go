package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/rs/zerolog"
)

type paymentsSummaryHandler struct {
	l                                                    zerolog.Logger
	dURL, fURL                                           string
	influxUrl, influxAdminToken, influxOrg, influxBucket string
}

type stats struct {
	TotalRequests int64   `json:"totalRequests"`
	TotalAmount   float64 `json:"totalAmount"`
}

type summaryPayload struct {
	Default  stats `json:"default"`
	Fallback stats `json:"fallback"`
}

func NewPaymentsSummaryHandler(
	l zerolog.Logger,
	dURL, fURL, iURL, iToken, iOrg, iBucket string,
) paymentsSummaryHandler {
	return paymentsSummaryHandler{
		l:                l,
		dURL:             dURL,
		fURL:             fURL,
		influxUrl:        iURL,
		influxAdminToken: iToken,
		influxOrg:        iOrg,
		influxBucket:     iBucket,
	}
}

func (h *paymentsSummaryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	influxClient := influxdb2.NewClient(h.influxUrl, h.influxAdminToken)
	defer influxClient.Close()
	queryAPI := influxClient.QueryAPI(h.influxOrg)

	runQuery := func(aggFn, requestType string) (int64, float64) {
		query := fmt.Sprintf(`
            from(bucket: "%s")
              |> range(start: -24h)
              |> filter(fn: (r) => r._measurement == "payments" and r.type == "%s")
              |> %s()
        `, h.influxBucket, requestType, aggFn)

		result, err := queryAPI.Query(context.Background(), query)
		if err != nil {
			log.Printf("Query failed for %s/%s: %v", requestType, aggFn, err)
			return 0, 0.0
		}

		if result.Next() {
			record := result.Record().Value()
			if record == nil {
				return 0, 0.0
			}
			switch v := record.(type) {
			case int64:
				return v, float64(v)
			case float64:
				return int64(v), v
			}
		}
		return 0, 0.0
	}

	_, defaultSum := runQuery("sum", "default")
	defaultCount, _ := runQuery("count", "default")

	_, fallbackSum := runQuery("sum", "fallback")
	fallbackCount, _ := runQuery("count", "fallback")

	response := summaryPayload{
		Default: stats{
			TotalRequests: defaultCount,
			TotalAmount:   defaultSum,
		},
		Fallback: stats{
			TotalRequests: fallbackCount,
			TotalAmount:   fallbackSum,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "failed to encode json response", http.StatusInternalServerError)
	}
}
