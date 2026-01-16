package main

import (
	"context"
	"io"
	"log"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/mgerstmannsf/insight-processing-platform/internal/adapters/inbound/lambda/ingest"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/readwise/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		req := events.APIGatewayV2HTTPRequest{
			Version:         "2.0",
			RouteKey:        "POST /readwise/webhook",
			RawPath:         r.URL.Path,
			RawQueryString:  r.URL.RawQuery,
			Headers:         map[string]string{},
			Body:            string(body),
			IsBase64Encoded: false,
		}

		// Copy headers (simple; API GW uses lowercased keys sometimes, but this is fine for local)
		for k, v := range r.Header {
			if len(v) > 0 {
				req.Headers[k] = v[0]
			}
		}

		resp, err := ingest.Handler(context.Background(), req)
		if err != nil {
			log.Printf("handler error: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		for k, v := range resp.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write([]byte(resp.Body))
	})

	addr := ":8080"
	log.Printf("listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
