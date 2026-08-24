package selfcheck

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

func jsonBody(value any) io.Reader {
	raw, _ := json.Marshal(value)
	return bytes.NewReader(raw)
}

func decodeAny(response *http.Response, target any) error {
	return json.NewDecoder(response.Body).Decode(target)
}
