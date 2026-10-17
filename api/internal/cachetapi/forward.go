package cachetapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
)

// forward runs a native Vigie handler with a translated JSON body and URL
// parameters, keeping the authenticated user and the client's address for
// the audit trail. It returns the status and decoded body.
func forward(c *gin.Context, h gin.HandlerFunc, method string, params gin.Params, body any) (int, map[string]any) {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequestWithContext(c.Request.Context(), method, c.Request.URL.Path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.Request.UserAgent())
	req.RemoteAddr = c.Request.RemoteAddr

	w := httptest.NewRecorder()
	inner, _ := gin.CreateTestContext(w)
	inner.Request = req
	inner.Params = params
	for k, v := range c.Keys {
		inner.Set(k, v)
	}
	h(inner)

	out := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

// forwardError turns a failed native response into a Cachet error, and
// reports whether there was one.
func forwardError(c *gin.Context, status int, body map[string]any) bool {
	if status < 300 {
		return false
	}
	msg, _ := body["error"].(string)
	if d, ok := body["details"].(string); ok && d != "" {
		msg += ": " + d
	}
	if msg == "" {
		msg = http.StatusText(status)
	}
	fail(c, status, msg)
	return true
}

// createdID reads the id of the object a native handler created or saved,
// under the given key.
func createdID(body map[string]any, key string) uint {
	obj, _ := body[key].(map[string]any)
	id, _ := obj["id"].(float64)
	return uint(id)
}
