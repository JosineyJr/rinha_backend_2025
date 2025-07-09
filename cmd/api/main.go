package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/JosineyJr/rinha_backend_2025/internal/handlers"
	"github.com/JosineyJr/rinha_backend_2025/internal/structs"
	"github.com/JosineyJr/rinha_backend_2025/internal/wizard"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/rs/zerolog"
)

var (
	PORT, PAYMENTS_PROCESSOR_URL_DEFAULT, PAYMENTS_PROCESSOR_URL_FALLBACK, SUMMARY_PROCESSOR_URL_DEFAULT                                                           string
	SUMMARY_PROCESSOR_URL_FALLBACK, HEALTH_PROCESSOR_URL_DEFAULT, HEALTH_PROCESSOR_URL_FALLBACK, INFLUXDB_ADMIN_TOKEN, INFLUXDB_ORG, INFLUXDB_BUCKET, INFLUXDB_URL string
	PAYMENT_PROCESSOR_TAX_DEFAULT, PAYMENT_PROCESSOR_TAX_FALLBACK                                                                                                  float32
)

func init() {
	PORT = os.Getenv("PORT")
	PAYMENTS_PROCESSOR_URL_DEFAULT = os.Getenv("PAYMENTS_PROCESSOR_URL_DEFAULT")
	PAYMENTS_PROCESSOR_URL_FALLBACK = os.Getenv("PAYMENTS_PROCESSOR_URL_FALLBACK")
	SUMMARY_PROCESSOR_URL_DEFAULT = os.Getenv("SUMMARY_PROCESSOR_URL_DEFAULT")
	SUMMARY_PROCESSOR_URL_FALLBACK = os.Getenv("SUMMARY_PROCESSOR_URL_FALLBACK")
	HEALTH_PROCESSOR_URL_DEFAULT = os.Getenv("HEALTH_PROCESSOR_URL_DEFAULT")
	HEALTH_PROCESSOR_URL_FALLBACK = os.Getenv("HEALTH_PROCESSOR_URL_FALLBACK")
	PAYMENT_PROCESSOR_TAX_DEFAULT = 0.05
	PAYMENT_PROCESSOR_TAX_FALLBACK = 0.15
	INFLUXDB_ADMIN_TOKEN = os.Getenv("INFLUXDB_ADMIN_TOKEN")
	INFLUXDB_ORG = os.Getenv("INFLUXDB_ORG")
	INFLUXDB_BUCKET = os.Getenv("INFLUXDB_BUCKET")
	INFLUXDB_URL = os.Getenv("INFLUXDB_URL")
}

func main() {
	logger := zerolog.New(
		zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339},
	).Level(zerolog.TraceLevel).With().Timestamp().Caller().Logger()

	paymentsSummaryHandler := handlers.NewPaymentsSummaryHandler(
		logger,
		INFLUXDB_URL,
		INFLUXDB_ADMIN_TOKEN,
		INFLUXDB_ORG,
		INFLUXDB_BUCKET,
		&logger,
	)

	pw := wizard.NewProcessorWizard(
		HEALTH_PROCESSOR_URL_DEFAULT,
		HEALTH_PROCESSOR_URL_FALLBACK,
		0.5,
	)
	pw.Listen(context.Background(), 5200*time.Millisecond)
	paymentsCh := make(chan *structs.PaymentsPayload, 30_000)

	for range 2 {
		go func() {
			influxClient := influxdb2.NewClient(INFLUXDB_URL, INFLUXDB_ADMIN_TOKEN)
			wApi := influxClient.WriteAPI(INFLUXDB_ORG, INFLUXDB_BUCKET)
			defer influxClient.Close()

			httpClient := http.Client{
				Transport: &http.Transport{
					MaxIdleConns:    500,
					IdleConnTimeout: 2 * time.Second,
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
				Timeout: 10 * time.Second,
			}

			for payment := range paymentsCh {
				payload, err := json.Marshal(payment)
				if err != nil {
					paymentsCh <- payment
					continue
				}

				tag := structs.DefaultProcessor
				if float64(
					pw.DefaultMinResponseTime.Load(),
				)*payment.Amount*float64(
					PAYMENT_PROCESSOR_TAX_DEFAULT,
				) <= float64(
					pw.FallbackMinResponseTime.Load(),
				)*payment.Amount*float64(
					PAYMENT_PROCESSOR_TAX_FALLBACK,
				)*float64(PAYMENT_PROCESSOR_TAX_DEFAULT/PAYMENT_PROCESSOR_TAX_FALLBACK) {
					_, err = httpClient.Post(
						PAYMENTS_PROCESSOR_URL_DEFAULT,
						"application/json",
						bytes.NewReader(payload),
					)
					if err != nil {
						paymentsCh <- payment
						continue
					}
				} else {
					tag = structs.FallbackProcessor
					_, err = httpClient.Post(
						PAYMENTS_PROCESSOR_URL_FALLBACK,
						"application/json",
						bytes.NewReader(payload),
					)
					if err != nil {
						paymentsCh <- payment
						continue
					}
				}

				p := influxdb2.NewPointWithMeasurement("payments").
					AddTag("type", tag).
					AddField("amount", payment.Amount).
					SetTime(payment.RequestedAt)
				wApi.WritePoint(p)
			}
		}()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /payments", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var payload structs.PaymentsPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			logger.Error().Err(err).Send()
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
		paymentsCh <- &payload

		w.WriteHeader(http.StatusAccepted)
	})
	mux.Handle("GET /payments-summary", &paymentsSummaryHandler)

	client := influxdb2.NewClient(INFLUXDB_URL, INFLUXDB_ADMIN_TOKEN)
	defer client.Close()

	deleteAPI := client.DeleteAPI()
	mux.HandleFunc("POST /admin/purge-payments", func(w http.ResponseWriter, r *http.Request) {
		start := time.Unix(0, 0)
		stop := time.Now().UTC()

		err := deleteAPI.DeleteWithName(
			context.Background(),
			INFLUXDB_ORG,
			INFLUXDB_BUCKET,
			start,
			stop,
			`_measurement="payments"`,
		)
		if err != nil {

			return
		}

		w.WriteHeader(http.StatusAccepted)
	})

	fmt.Println("server running")
	if err := http.ListenAndServe(":"+PORT, mux); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
