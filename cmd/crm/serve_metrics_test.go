package main

import (
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestRunServeMetricsAddressOccupied(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	values := validEnv()
	values["METRICS_ADDR"] = occupied.Addr().String()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err = run(ctx, []string{"serve"}, mapEnv(values), io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "METRICS_ADDR") {
		t.Fatalf("serve with occupied METRICS_ADDR: %v", err)
	}
}
