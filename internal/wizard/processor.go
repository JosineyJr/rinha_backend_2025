package wizard

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/JosineyJr/rinha_backend_2025/internal/structs"
)

type ProcessorWizard struct {
	defaultHealthEndpoint   string
	fallbackHealthEndpoint  string
	DefaultMinResponseTime  atomic.Int32
	FallbackMinResponseTime atomic.Int32
	IsDefaultFailing        atomic.Bool
	IsFallbackFailing       atomic.Bool
	httpClient              http.Client
}

func NewProcessorWizard(dhe, fhe string) ProcessorWizard {
	return ProcessorWizard{
		httpClient: http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		},
		defaultHealthEndpoint:  dhe,
		fallbackHealthEndpoint: fhe,
	}
}

func (pw *ProcessorWizard) Listen(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	var shp structs.ServiceHealthPayload

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				resp, err := pw.httpClient.Get(pw.defaultHealthEndpoint)
				if err != nil {
					pw.IsDefaultFailing.Store(true)
				}
				b, err := io.ReadAll(resp.Body)
				if err != nil {
					pw.IsDefaultFailing.Store(true)
				}
				err = json.Unmarshal(b, &shp)
				if err != nil {
					pw.IsDefaultFailing.Store(true)
				}
				pw.IsDefaultFailing.Store(shp.Failing)
				pw.DefaultMinResponseTime.Store(int32(shp.MinResponseTime))
				resp.Body.Close()

				resp, err = pw.httpClient.Get(pw.fallbackHealthEndpoint)
				if err != nil {
					pw.IsFallbackFailing.Store(true)
				}
				b, err = io.ReadAll(resp.Body)
				if err != nil {
					pw.IsFallbackFailing.Store(true)
				}
				err = json.Unmarshal(b, &shp)
				if err != nil {
					pw.IsFallbackFailing.Store(true)
				}
				pw.IsFallbackFailing.Store(shp.Failing)
				pw.FallbackMinResponseTime.Store(int32(shp.MinResponseTime))
				resp.Body.Close()

				continue
			}
		}
	}()

}
