package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/rs/zerolog"
)

type paymentHandler struct {
	l zerolog.Logger
}

type paymentsPayload struct {
	CorrelationID string  `json:"correlationId"`
	Amount        float64 `json:"amount"`
}

func NewPaymentHandler(l zerolog.Logger) paymentHandler {
	return paymentHandler{
		l: l,
	}
}

func (h *paymentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		h.l.Error().Err(err).Send()
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message": "error on read body"}`))
		return
	}

	var payload paymentsPayload
	err = json.Unmarshal(bodyBytes, &payload)
	if err != nil {
		h.l.Error().Err(err).Send()
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message": "error on unmarshal body bytes"}`))
		return
	}

	if payload.CorrelationID == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message": "missing field 'correlationId'"}`))
		return
	}

	if payload.Amount == 0.0 {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message": "missing field 'amount'"}`))
		return
	}
}
