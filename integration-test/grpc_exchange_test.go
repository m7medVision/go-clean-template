package integration_test

import (
	"testing"

	protov1 "github.com/m7medvision/go-clean-template/docs/proto/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

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
		&protov1.ConvertRequest{From: "OMR", To: "CNY", Amount: "100.10"})
	if err != nil {
		t.Fatal("gRPC Client - remote call error - client.Convert", err)
	}

	if conversion.GetResult() != "1742.39" {
		t.Errorf("Expected result 1742.39, got %s", conversion.GetResult())
	}

	_, err = client.Convert(grpcAuthCtx(t, token), &protov1.ConvertRequest{From: "XXX", To: "CNY", Amount: "1"})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("Expected InvalidArgument for an unsupported currency, got %v", err)
	}

	history, err := client.GetHistory(grpcAuthCtx(t, token), &protov1.GetExchangeHistoryRequest{})
	if err != nil {
		t.Fatal("gRPC Client - remote call error - client.GetHistory", err)
	}

	if len(history.GetHistory()) != 1 || history.GetTotal() != 1 {
		t.Errorf("Expected 1 conversion in history and total 1, got %d and %d",
			len(history.GetHistory()), history.GetTotal())
	}
}
