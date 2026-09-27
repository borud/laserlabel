package laserlabel

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithBasePath(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.URL.Path))
	})

	for _, tc := range []struct {
		base, path string
		code       int
		body       string
	}{
		{"", "/api/fonts", 200, "/api/fonts"},
		{"/laser", "/laser/api/fonts", 200, "/api/fonts"},
		{"laser/", "/laser/", 200, "/"},
		{"/laser", "/laser", 301, ""},
		{"/laser", "/other", 404, ""},
	} {
		rec := httptest.NewRecorder()
		withBasePath(tc.base, inner).ServeHTTP(rec, httptest.NewRequest("GET", tc.path, nil))
		if rec.Code != tc.code {
			t.Errorf("base %q path %q: code %d, want %d", tc.base, tc.path, rec.Code, tc.code)
		}
		if tc.body != "" && rec.Body.String() != tc.body {
			t.Errorf("base %q path %q: body %q, want %q", tc.base, tc.path, rec.Body.String(), tc.body)
		}
	}
}
