package integration_test

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	protov1 "github.com/evrone/go-clean-template/docs/proto/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// The app's EXCHANGE_SERVICE_URL points at integration-test/exchange-stub, which answers every
// rate request with the recorded EUR→USD response (1.1225 on 2026-10-19), and with a 404 for XXX.

type conversionBody struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Amount   string `json:"amount"`
	Rate     string `json:"rate"`
	Result   string `json:"result"`
	RateDate string `json:"rate_date"`
}

func postConvert(t *testing.T, token, body string) *http.Response {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()

	resp, err := doAuthenticatedRequest(ctx, http.MethodPost, basePathV1+"/exchange/convert",
		bytes.NewBufferString(body), token)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	return resp
}

// HTTP POST: /v1/exchange/convert.
func TestHTTPConvertV1(t *testing.T) {
	token := registerAndLogin(t)

	t.Run("converts at the stub's rate", func(t *testing.T) {
		resp := postConvert(t, token, `{"from":"eur","to":"usd","amount":"100.10"}`)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
		}

		got := parseJSON[conversionBody](t, resp)
		want := conversionBody{
			From: "EUR", To: "USD", Amount: "100.1", Rate: "1.1225", Result: "112.36", RateDate: "2026-10-19",
		}

		if got != want {
			t.Errorf("Expected %+v, got %+v", want, got)
		}
	})

	for _, tt := range []struct {
		description string
		body        string
		expected    int
	}{
		{"unsupported currency", `{"from":"EUR","to":"XXX","amount":"1"}`, http.StatusBadRequest},
		{"same currency", `{"from":"EUR","to":"eur","amount":"1"}`, http.StatusBadRequest},
		{"malformed amount", `{"from":"EUR","to":"USD","amount":"-1"}`, http.StatusUnprocessableEntity},
	} {
		t.Run(tt.description, func(t *testing.T) {
			resp := postConvert(t, token, tt.body)
			defer resp.Body.Close()

			if resp.StatusCode != tt.expected {
				t.Errorf("Expected status %d, got %d", tt.expected, resp.StatusCode)
			}
		})
	}
}

// HTTP GET: /v1/exchange/history.
func TestHTTPExchangeHistoryV1(t *testing.T) {
	token := registerAndLogin(t)

	resp := postConvert(t, token, `{"from":"EUR","to":"USD","amount":"5"}`)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status %d for the conversion, got %d", http.StatusOK, resp.StatusCode)
	}

	ctx, cancel := context.WithTimeout(t.Context(), requestTimeout)
	defer cancel()

	resp, err := doAuthenticatedRequest(ctx, http.MethodGet, basePathV1+"/exchange/history", http.NoBody, token)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	body := parseJSON[struct {
		History []conversionBody `json:"history"`
	}](t, resp)

	if len(body.History) != 1 || body.History[0].Result != "5.61" {
		t.Errorf("Expected one conversion with result 5.61, got %+v", body.History)
	}
}

// gRPC Client V1: Convert and GetHistory.
func TestGRPCExchangeV1(t *testing.T) {
	token := registerAndLoginGRPC(t)

	grpcConn, err := grpc.NewClient(grpcURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal("gRPC Client - init error - grpc.NewClient", err)
	}

	defer func() {
		if cerr := grpcConn.Close(); cerr != nil {
			t.Fatal("gRPC Client - shutdown error - grpcConn.Close", cerr)
		}
	}()

	client := protov1.NewExchangeServiceClient(grpcConn)

	conversion, err := client.Convert(grpcAuthCtx(t, token),
		&protov1.ConvertRequest{From: "EUR", To: "USD", Amount: "100.10"})
	if err != nil {
		t.Fatal("gRPC Client - remote call error - client.Convert", err)
	}

	if conversion.GetResult() != "112.36" {
		t.Errorf("Expected result 112.36, got %s", conversion.GetResult())
	}

	_, err = client.Convert(grpcAuthCtx(t, token), &protov1.ConvertRequest{From: "EUR", To: "XXX", Amount: "1"})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("Expected InvalidArgument for an unsupported currency, got %v", err)
	}

	history, err := client.GetHistory(grpcAuthCtx(t, token), &protov1.GetExchangeHistoryRequest{})
	if err != nil {
		t.Fatal("gRPC Client - remote call error - client.GetHistory", err)
	}

	if len(history.GetHistory()) != 1 {
		t.Errorf("Expected 1 conversion in history, got %d", len(history.GetHistory()))
	}
}
