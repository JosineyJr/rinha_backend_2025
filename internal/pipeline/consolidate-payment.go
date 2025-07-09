package pipeline

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"time"

	"github.com/JosineyJr/rinha_backend_2025/internal/structs"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/rs/zerolog"
)

func ConsolidatePayment(
	paymentsCh <-chan structs.ConsolidatePayment,
	influxUrl, influxAdminToken, influxOrg, influxBucket string,
	logger *zerolog.Logger,
) {
	httpClient := http.Client{
		Transport: &http.Transport{
			MaxIdleConns:    500,
			IdleConnTimeout: 10 * time.Second,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: 15 * time.Second,
	}

	influxClient := influxdb2.NewClient(influxUrl, influxAdminToken)

	go func() {
		defer influxClient.Close()

		wApi := influxClient.WriteAPI(influxOrg, influxBucket)

		for consolidate := range paymentsCh {

			payload, err := json.Marshal(consolidate.Payload)
			if err != nil {
				logger.Error().Err(err).Send()
				return
			}

			_, err = httpClient.Post(
				consolidate.ProcessorURL,
				"application/json",
				bytes.NewReader(payload),
			)
			if err != nil {
				logger.Error().Err(err).Send()
				continue
			}

			p := influxdb2.NewPointWithMeasurement("payments").
				AddTag("type", consolidate.Tag).
				AddField("amount", consolidate.Payload.Amount).
				SetTime(consolidate.Payload.RequestedAt)
			wApi.WritePoint(p)
			wApi.Flush()

			// logger.Info().
			// 	Str("processor", consolidate.ProcessorURL).
			// 	Msg("payment consolidated")
		}
	}()
}
