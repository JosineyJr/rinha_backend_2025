package handlers

import (
	"net/http"

	"github.com/rs/zerolog"
)

type paymentsSummaryHandler struct {
	l zerolog.Logger
}

func NewPaymentsSummaryHandler(l zerolog.Logger) paymentsSummaryHandler {
	return paymentsSummaryHandler{l: l}
}

func (h *paymentsSummaryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
}
