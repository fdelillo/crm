package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

const maxJSONBytes = 64 * 1024

// DecodeJSON enforces the API's JSON media type and 64 KiB body limit. The root must be a JSON object: null,
// an array and a scalar are malformed_request, because encoding/json would otherwise accept null for a struct
// and leave it empty (plan 9.1).
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		WriteProblem(w, r, CodeUnsupportedMediaType)
		return errors.New("json content type required")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return decodeError(w, r, err)
	}
	if !bytes.HasPrefix(bytes.TrimLeft(body, " \t\r\n"), []byte("{")) {
		return decodeError(w, r, errors.New("json root is not an object"))
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return decodeError(w, r, err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return decodeError(w, r, err)
	}
	return nil
}
func decodeError(w http.ResponseWriter, r *http.Request, err error) error {
	var max *http.MaxBytesError
	if errors.As(err, &max) {
		WriteProblem(w, r, CodePayloadTooLarge)
	} else {
		WriteProblem(w, r, CodeMalformedRequest)
	}
	return err
}
