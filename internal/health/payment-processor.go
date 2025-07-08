package health

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

var errConnectionIll = errors.New("payment processor is failing")

func CheckPaymentProcessor(ctx context.Context, url string) func(context.Context) error {
	hc := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	type response struct {
		Failing         bool `json:"failing"`
		MinResponseTime int  `json:"minResponseTime"`
	}

	return func(ctx context.Context) error {
		resp, err := hc.Get(url)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		var r response
		err = json.Unmarshal(b, &r)
		if err != nil {
			return err
		}

		if r.Failing {
			return errConnectionIll
		}

		return nil
	}
}
