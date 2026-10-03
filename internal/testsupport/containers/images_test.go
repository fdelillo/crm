package containers

import (
	"os"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestComposeImagesMatchIntegrationTests(t *testing.T) {
	data, err := os.ReadFile("../../../compose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var compose struct {
		Services map[string]struct {
			Image string `yaml:"image"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &compose); err != nil {
		t.Fatal(err)
	}
	for service, want := range map[string]string{"postgres": Postgres, "mailpit": Mailpit, "minio": MinIO} {
		if got := compose.Services[service].Image; got != want {
			t.Errorf("%s image = %q, want %q", service, got, want)
		}
	}
	if !strings.HasPrefix(MinIO, "ghcr.io/coollabsio/minio:RELEASE.2025-10-15T17-29-55Z@sha256:") {
		t.Fatalf("MinIO image is not pinned: %s", MinIO)
	}
}
