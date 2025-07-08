package handlers

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"time"

	"github.com/JosineyJr/rinha_backend_2025/internal/health"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/rs/zerolog"
)

type paymentHandler struct {
	l                                                    zerolog.Logger
	dURL, fURL                                           string
	http                                                 http.Client
	hc                                                   *health.HealthChecker
	influxUrl, influxAdminToken, influxOrg, influxBucket string
}

type paymentsPayload struct {
	CorrelationID string    `json:"correlationId"`
	Amount        float64   `json:"amount"`
	RequestedAt   time.Time `json:"requestedAt"`
}

var (
	dProcessor string = "default"
	fProcessor string = "fallback"
)

func NewPaymentHandler(
	l zerolog.Logger,
	dURL, fURL string,
	hc *health.HealthChecker,
	iURL, iToken, iOrg, iBucket string,
) paymentHandler {
	return paymentHandler{
		l:                l,
		dURL:             dURL,
		fURL:             fURL,
		hc:               hc,
		influxUrl:        iURL,
		influxBucket:     iBucket,
		influxOrg:        iOrg,
		influxAdminToken: iToken,
		http: http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		},
	}
}

func (h *paymentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var payload paymentsPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		h.l.Error().Err(err).Send()
		return
	}

	if payload.CorrelationID == "" {
		http.Error(w, "missing field 'correlationId'", http.StatusBadRequest)
		return
	}

	if payload.Amount == 0.0 {
		http.Error(w, "missing field 'amount'", http.StatusBadRequest)
		return
	}
	payload.RequestedAt = time.Now().UTC()

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		h.l.Error().Err(err).Send()
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	url := h.dURL
	tag := dProcessor
	if h.hc.IsIll {
		if payload.Amount > 100 {
			h.hc.WaitForHealing()
		}
		url = h.fURL
		tag = fProcessor
	}

	payloadBuffer := bytes.NewBuffer(payloadBytes)
	res, err := h.http.Post(url, "application/json", payloadBuffer)
	if err != nil {
		h.l.Error().Err(err).Send()
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	influxClient := influxdb2.NewClient(h.influxUrl, h.influxAdminToken)
	defer influxClient.Close()

	wApi := influxClient.WriteAPI(h.influxOrg, h.influxBucket)
	p := influxdb2.NewPointWithMeasurement("payments").
		AddTag("type", tag).
		AddField("amount", payload.Amount).
		SetTime(payload.RequestedAt)
	wApi.WritePoint(p)

	w.WriteHeader(res.StatusCode)
}
