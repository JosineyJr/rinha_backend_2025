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
	DefaultLatencyEWMA      atomic.Int32
	FallbackLatencyEWMA     atomic.Int32
	Alpha                   float64
}

func NewProcessorWizard(dhe, fhe string, alpha float64) ProcessorWizard {
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
		Alpha:                  alpha,
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
					continue
				}
				b, err := io.ReadAll(resp.Body)
				if err != nil {
					pw.IsDefaultFailing.Store(true)
					resp.Body.Close()
					continue
				}
				err = json.Unmarshal(b, &shp)
				if err != nil {
					pw.IsDefaultFailing.Store(true)
					resp.Body.Close()
					continue
				}
				pw.IsDefaultFailing.Store(shp.Failing)
				pw.DefaultMinResponseTime.Store(int32(shp.MinResponseTime))
				if pw.DefaultLatencyEWMA.Load() == 0 {
					pw.DefaultLatencyEWMA.Store(int32(shp.MinResponseTime))
				} else {
					pw.DefaultLatencyEWMA.Store(
						ewma(pw.DefaultLatencyEWMA.Load(), int32(shp.MinResponseTime), pw.Alpha),
					)
				}
				resp.Body.Close()

				resp, err = pw.httpClient.Get(pw.fallbackHealthEndpoint)
				if err != nil {
					pw.IsFallbackFailing.Store(true)
					continue
				}
				b, err = io.ReadAll(resp.Body)
				if err != nil {
					pw.IsFallbackFailing.Store(true)
					resp.Body.Close()
					continue
				}
				err = json.Unmarshal(b, &shp)
				if err != nil {
					pw.IsFallbackFailing.Store(true)
					resp.Body.Close()
					continue
				}
				pw.IsFallbackFailing.Store(shp.Failing)
				pw.FallbackMinResponseTime.Store(int32(shp.MinResponseTime))
				if pw.FallbackLatencyEWMA.Load() == 0 {
					pw.FallbackLatencyEWMA.Store(int32(shp.MinResponseTime))
				} else {
					pw.FallbackLatencyEWMA.Store(
						ewma(pw.FallbackLatencyEWMA.Load(), int32(shp.MinResponseTime), pw.Alpha),
					)
				}
				resp.Body.Close()

				continue
			}
		}
	}()

}

func ewma(prev, new int32, alpha float64) int32 {
	return int32(alpha*float64(new) + (1-alpha)*float64(prev))
}
