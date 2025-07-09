package pipeline

import (
	"context"

	"github.com/JosineyJr/rinha_backend_2025/internal/structs"
	"github.com/JosineyJr/rinha_backend_2025/internal/wizard"
)

func ChooseProcessor(
	ctx context.Context,
	paymentsCh <-chan *structs.PaymentsPayload,
	pw *wizard.ProcessorWizard,
	dTax, fTax float32,
	dpe, fpe string,
) (chan structs.ConsolidatePayment, chan structs.ConsolidatePayment) {
	d, f := make(
		chan structs.ConsolidatePayment,
		9000,
	), make(
		chan structs.ConsolidatePayment,
		9000,
	)

	// logger := zerolog.New(
	// 	zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339},
	// ).Level(zerolog.TraceLevel).With().Timestamp().Caller().Logger()

	go func() {
		defer close(d)
		defer close(f)

		for payment := range paymentsCh {
			dPoints := float64(pw.DefaultMinResponseTime.Load()) * payment.Amount * float64(dTax)
			fPoints := float64(pw.FallbackMinResponseTime.Load()) * payment.Amount * float64(fTax)

			if dPoints <= fPoints*3 {
				// d <- structs.ConsolidatePayment{
				// 	ProcessorURL: dpe,
				// 	Payload:      payment,
				// 	Tag:          structs.DefaultProcessor,
				// }
				continue
			}

			// f <- structs.ConsolidatePayment{
			// 	ProcessorURL: fpe,
			// 	Payload:      payment,
			// 	Tag:          structs.FallbackProcessor,
			// }
		}

	}()

	return d, f
}
