package main

import (
	"bytes"
	"errors"
	"github.com/fdelillo/crm/internal/tenant"
	"strings"
	"testing"
)

func TestReprovisionReportsFailedIDsAndReturnsFailure(t *testing.T) {
	var output bytes.Buffer
	failure := errors.New("forced middle failure")
	err := finishReprovision(&output, tenant.ReprovisionReport{Tenants: 3, Failed: []string{"middle-id"}}, failure)
	if !errors.Is(err, failure) || !strings.Contains(output.String(), `"tenants":3`) || !strings.Contains(output.String(), "middle-id") {
		t.Fatalf("output=%s err=%v", &output, err)
	}
}
