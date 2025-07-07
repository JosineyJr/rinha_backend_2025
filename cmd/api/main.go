package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/JosineyJr/rinha_backend_2025/internal/handlers"
	"github.com/rs/zerolog"
)

func init() {

}

func main() {
	logger := zerolog.New(
		zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339},
	).Level(zerolog.TraceLevel).With().Timestamp().Caller().Logger()

	paymentsHandler := handlers.NewPaymentHandler(logger)
	paymentsSummaryHandler := handlers.NewPaymentsSummaryHandler(logger)

	mux := http.NewServeMux()
	mux.Handle("POST /payments", &paymentsHandler)
	mux.Handle("GET /payments-summary", &paymentsSummaryHandler)

	fmt.Println("server running")
	if err := http.ListenAndServe(":9999", mux); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
