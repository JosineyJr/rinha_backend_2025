package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/JosineyJr/rinha_backend_2025/internal/structs"
	"github.com/JosineyJr/rinha_backend_2025/internal/wizard"
	"github.com/rs/zerolog"
)

func ChooseProcessor(
	ctx context.Context,
	paymentsCh <-chan structs.PaymentsPayload,
	pw *wizard.ProcessorWizard,
	dTax, fTax float32,
	dpe, fpe string,
) (chan structs.ConsolidatePayment, chan structs.ConsolidatePayment) {
	d, f := make(
		chan structs.ConsolidatePayment,
		1000,
	), make(
		chan structs.ConsolidatePayment,
		1000,
	)

	logger := zerolog.New(
		zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339},
	).Level(zerolog.TraceLevel).With().Timestamp().Caller().Logger()

	go func() {
		defer close(d)
		defer close(f)

		for payment := range paymentsCh {
			dPoints := float64(pw.DefaultMinResponseTime.Load()) * payment.Amount * float64(dTax)
			fPoints := float64(pw.FallbackMinResponseTime.Load()) * payment.Amount * float64(fTax)

			payload, err := json.Marshal(payment)
			if err != nil {
				logger.Error().Err(err).Send()
				return
			}

			if dPoints <= fPoints*3 {
				d <- structs.ConsolidatePayment{
					ProcessorURL: dpe,
					Payload:      bytes.NewBuffer(payload),
					Tag:          structs.DefaultProcessor,
					Amount:       payment.Amount,
					RequestedAt:  payment.RequestedAt,
				}
				continue
			}

			f <- structs.ConsolidatePayment{
				ProcessorURL: fpe,
				Payload:      bytes.NewBuffer(payload),
				Tag:          structs.FallbackProcessor,
				Amount:       payment.Amount,
				RequestedAt:  payment.RequestedAt,
			}
		}

	}()

	return d, f
}
