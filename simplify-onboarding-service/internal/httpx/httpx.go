// Package httpx contains small helpers for JSON request/response handling,
// keeping handlers free of boilerplate and consistent in their error envelope.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// ErrBodyTooLarge is returned by DecodeJSON when the request body exceeds the limit.
var ErrBodyTooLarge = errors.New("request body too large")

const maxBody = 1 << 20 // 1 MiB

// WriteJSON serialises v as JSON with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

// ErrorBody is the canonical error envelope returned to the browser/client.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteError writes a structured error the frontend api.ts can map to copy.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, ErrorBody{Code: code, Message: message})
}

// DecodeJSON reads and strictly decodes a JSON request body into dst.
func DecodeJSON(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBody)
	dec := json.NewDecoder(r.Body)
	// Lenient on unknown fields — the SPA may send extra UI-only fields
	// (e.g. confirmPassword) we deliberately ignore server-side.
	if err := dec.Decode(dst); err != nil {
		var mbErr *http.MaxBytesError
		if errors.As(err, &mbErr) {
			return ErrBodyTooLarge
		}
		return err
	}
	// Reject trailing data after the first JSON object.
	if dec.More() {
		_, _ = io.Copy(io.Discard, r.Body)
		return errors.New("unexpected extra JSON")
	}
	return nil
}
