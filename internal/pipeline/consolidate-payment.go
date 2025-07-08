package pipeline

import (
	"crypto/tls"
	"net/http"
	"os"
	"time"

	"github.com/JosineyJr/rinha_backend_2025/internal/structs"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/rs/zerolog"
)

func ConsolidatePayment(
	paymentsCh <-chan structs.ConsolidatePayment,
	influxUrl, influxAdminToken, influxOrg, influxBucket string,
) {
	httpClient := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	logger := zerolog.New(
		zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339},
	).Level(zerolog.TraceLevel).With().Timestamp().Caller().Logger()

	influxClient := influxdb2.NewClient(influxUrl, influxAdminToken)

	go func() {
		defer influxClient.Close()

		for consolidate := range paymentsCh {
			_, err := httpClient.Post(
				consolidate.ProcessorURL,
				"application/json",
				consolidate.Payload,
			)
			if err != nil {
				logger.Error().Err(err).Send()
				continue
			}

			wApi := influxClient.WriteAPI(influxOrg, influxBucket)
			p := influxdb2.NewPointWithMeasurement("payments").
				AddTag("type", consolidate.Tag).
				AddField("amount", consolidate.Amount).
				SetTime(consolidate.RequestedAt)
			wApi.WritePoint(p)

			logger.Info().
				Str("message", "payment consolidated").
				Str("processor", consolidate.ProcessorURL).
				Send()
		}
	}()
}
