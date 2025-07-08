package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/JosineyJr/rinha_backend_2025/internal/handlers"
	"github.com/JosineyJr/rinha_backend_2025/internal/pipeline"
	"github.com/JosineyJr/rinha_backend_2025/internal/structs"
	"github.com/JosineyJr/rinha_backend_2025/internal/wizard"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/rs/zerolog"
	_ "go.uber.org/automaxprocs"
)

var (
	PORT, PAYMENT_PROCESSOR_URL_DEFAULT, PAYMENT_PROCESSOR_URL_FALLBACK, INFLUXDB_ADMIN_TOKEN, INFLUXDB_ORG, INFLUXDB_BUCKET, INFLUXDB_URL string
	PAYMENT_PROCESSOR_TAX_DEFAULT, PAYMENT_PROCESSOR_TAX_FALLBACK                                                                          float32
)

func init() {
	PORT = os.Getenv("PORT")
	PAYMENT_PROCESSOR_URL_DEFAULT = os.Getenv("PAYMENT_PROCESSOR_URL_DEFAULT")
	PAYMENT_PROCESSOR_URL_FALLBACK = os.Getenv("PAYMENT_PROCESSOR_URL_FALLBACK")
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)

	paymentsSummaryHandler := handlers.NewPaymentsSummaryHandler(
		logger,
		PAYMENT_PROCESSOR_URL_DEFAULT+"/payments-summary",
		PAYMENT_PROCESSOR_URL_FALLBACK+"/payments-summary",
		INFLUXDB_URL,
		INFLUXDB_ADMIN_TOKEN,
		INFLUXDB_ORG,
		INFLUXDB_BUCKET,
		&logger,
	)

	pw := wizard.NewProcessorWizard(
		PAYMENT_PROCESSOR_URL_DEFAULT+"/payments/service-health",
		PAYMENT_PROCESSOR_URL_FALLBACK+"/payments/service-health",
		0.5,
	)
	pw.Listen(ctx, 5200*time.Millisecond)
	paymentsCh := make(chan structs.PaymentsPayload, 5000)

	go func() {
		<-ctx.Done()
		stop()
		close(paymentsCh)
		logger.Info().Msg("program exit")
	}()

	for range 2 {
		go func() {
			defaultProcessorCh, fallbackProcessorCh := pipeline.ChooseProcessor(
				ctx,
				paymentsCh,
				&pw,
				PAYMENT_PROCESSOR_TAX_DEFAULT,
				PAYMENT_PROCESSOR_TAX_FALLBACK,
				PAYMENT_PROCESSOR_URL_DEFAULT+"/payments",
				PAYMENT_PROCESSOR_URL_FALLBACK+"/payments",
			)

			pipeline.ConsolidatePayment(
				defaultProcessorCh,
				INFLUXDB_URL,
				INFLUXDB_ADMIN_TOKEN,
				INFLUXDB_ORG,
				INFLUXDB_BUCKET,
				&logger,
			)
			pipeline.ConsolidatePayment(fallbackProcessorCh,
				INFLUXDB_URL,
				INFLUXDB_ADMIN_TOKEN,
				INFLUXDB_ORG,
				INFLUXDB_BUCKET,
				&logger,
			)
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
		paymentsCh <- payload

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
