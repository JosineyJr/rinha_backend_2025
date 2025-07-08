package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

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
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if fromStr == "" || toStr == "" {
		http.Error(w, "query parameters 'from' and 'to' are required.", http.StatusBadRequest)
		return
	}
	if _, err := time.Parse(time.RFC3339, fromStr); err != nil {
		http.Error(
			w,
			"invalid 'from' time format, use RFC3339 (e.g., 2020-07-10T12:34:56.000Z)",
			http.StatusBadRequest,
		)
		return
	}
	if _, err := time.Parse(time.RFC3339, toStr); err != nil {
		http.Error(
			w,
			"invalid 'to' time format, use RFC3339 (e.g., 2020-07-10T12:34:56.000Z)",
			http.StatusBadRequest,
		)
		return
	}

	influxClient := influxdb2.NewClient(h.influxUrl, h.influxAdminToken)
	defer influxClient.Close()
	queryAPI := influxClient.QueryAPI(h.influxOrg)

	runQuery := func(aggFn, requestType, from, to string) (int64, float64) {
		query := fmt.Sprintf(`
        from(bucket: "%s")
          |> range(start: %s, stop: %s)
          |> filter(fn: (r) => r._measurement == "payments" and r.type == "%s")
          |> %s()
    `, h.influxBucket, from, to, requestType, aggFn)

		result, err := queryAPI.Query(context.Background(), query)
		if err != nil {
			log.Printf("query failed for %s/%s: %v", requestType, aggFn, err)
			return 0, 0.0
		}
		if result.Next() && result.Record().Value() != nil {
			switch v := result.Record().Value().(type) {
			case int64:
				return v, float64(v)
			case float64:
				return int64(v), v
			}
		}
		return 0, 0.0
	}

	_, defaultSum := runQuery("sum", "default", fromStr, toStr)
	defaultCount, _ := runQuery("count", "default", fromStr, toStr)

	_, fallbackSum := runQuery("sum", "fallback", fromStr, toStr)
	fallbackCount, _ := runQuery("count", "fallback", fromStr, toStr)

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
