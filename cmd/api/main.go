package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/JosineyJr/rinha_backend_2025/internal/handlers"
	"github.com/JosineyJr/rinha_backend_2025/internal/health"
	"github.com/rs/zerolog"
)

var (
	PAYMENT_PROCESSOR_URL_DEFAULT, PAYMENT_PROCESSOR_URL_FALLBACK, INFLUXDB_ADMIN_TOKEN, INFLUXDB_ORG, INFLUXDB_BUCKET, INFLUXDB_URL string
)

func init() {
	PAYMENT_PROCESSOR_URL_DEFAULT = os.Getenv("PAYMENT_PROCESSOR_URL_DEFAULT")
	PAYMENT_PROCESSOR_URL_FALLBACK = os.Getenv("PAYMENT_PROCESSOR_URL_FALLBACK")
	INFLUXDB_ADMIN_TOKEN = os.Getenv("INFLUXDB_ADMIN_TOKEN")
	INFLUXDB_ORG = os.Getenv("INFLUXDB_ORG")
	INFLUXDB_BUCKET = os.Getenv("INFLUXDB_BUCKET")
	INFLUXDB_URL = os.Getenv("INFLUXDB_URL")
}

func main() {
	logger := zerolog.New(
		zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339},
	).Level(zerolog.TraceLevel).With().Timestamp().Caller().Logger()

	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)

	defaultProcessorHc := health.NewHealthChecker(
		ctx,
		5*time.Second,
		health.CheckPaymentProcessor(ctx, PAYMENT_PROCESSOR_URL_DEFAULT+"/payments/service-health"),
		logger,
	)

	fallbackProcessorHc := health.NewHealthChecker(
		ctx,
		5*time.Second,
		health.CheckPaymentProcessor(
			ctx,
			PAYMENT_PROCESSOR_URL_FALLBACK+"/payments/service-health",
		),
		logger,
	)

	paymentsHandler := handlers.NewPaymentHandler(
		logger,
		PAYMENT_PROCESSOR_URL_DEFAULT+"/payments",
		PAYMENT_PROCESSOR_URL_FALLBACK+"/payments",
		defaultProcessorHc,
		fallbackProcessorHc,
		INFLUXDB_URL,
		INFLUXDB_ADMIN_TOKEN,
		INFLUXDB_ORG,
		INFLUXDB_BUCKET,
	)
	paymentsSummaryHandler := handlers.NewPaymentsSummaryHandler(
		logger,
		PAYMENT_PROCESSOR_URL_DEFAULT+"/payments-summary",
		PAYMENT_PROCESSOR_URL_FALLBACK+"/payments-summary",
		INFLUXDB_URL,
		INFLUXDB_ADMIN_TOKEN,
		INFLUXDB_ORG,
		INFLUXDB_BUCKET,
	)

	mux := http.NewServeMux()
	mux.Handle("POST /payments", &paymentsHandler)
	mux.Handle("GET /payments-summary", &paymentsSummaryHandler)

	fmt.Println("server running")
	if err := http.ListenAndServe(":9999", mux); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
