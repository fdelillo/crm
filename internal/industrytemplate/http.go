package industrytemplate

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RegisterRoutes makes the catalog public without requiring a session.
func RegisterRoutes(r chi.Router) {
	r.Get("/api/v1/industry-templates", func(w http.ResponseWriter, _ *http.Request) {
		items := make([]struct {
			Code string `json:"code"`
			Name string `json:"name"`
		}, 0, len(All()))
		for _, t := range All() {
			items = append(items, struct {
				Code string `json:"code"`
				Name string `json:"name"`
			}{Code: t.Code, Name: t.Name})
		}
		body, err := json.Marshal(struct {
			Items any `json:"items"`
		}{Items: items})
		if err != nil {
			panic(err) // the embedded catalog was validated before serving
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(body)
	})
}
