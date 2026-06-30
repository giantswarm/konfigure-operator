package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

const (
	konfSchema = `variables:
  - name: app
    required: true
layers:
  - id: defaults
    path:
      directory: default
      required: true
    values:
      path:
        directory: ""
      configMap:
        name: config.yaml
        required: true
    templates:
      path:
        directory: apps/<< app >>
        required: true
      configMap:
        name: configmap-values.yaml.template
        required: true
        values:
          merge:
            strategy: ConfigMapsInLayerOrder
includes: []`

	malformedKonfSchema = `variables:
  - name: app
    requirXXed: true
layers:
  - id: defaults
    pathX:
      directory: default
      required: true
    valuXXes:
      path:
        directory: ""
      configMap:
        name: config.yaml
        required: true
    templates:
      path:
        directory: apps/<< app >>
        required: true
      configMap:
        name: configmap-values.yaml.template
        required: true
        values:
          merge:
            strategy: ConfigMapsInLayerOrder
includes: []`
)

func TestFetchKonfigurationSchemaFromUrl(t *testing.T) {
	err := os.Mkdir(KonfigurationSchemaDir, 0750)
	if err != nil && !os.IsExist(err) {
		t.Fatalf("error creating %v directory: %v", KonfigurationSchemaDir, err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/schema-good":
			w.WriteHeader(http.StatusOK)
			_, err = fmt.Fprint(w, konfSchema)
			if err != nil {
				t.Fatalf("error writing response: %v", err)
			}
		case "/schema-malformed":
			w.WriteHeader(http.StatusOK)
			_, err = fmt.Fprint(w, malformedKonfSchema)
			if err != nil {
				t.Fatalf("error writing response: %v", err)
			}
		case "/schema-empty":
			w.WriteHeader(http.StatusOK)
		case "/schema-service-unavailable":
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	closedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedServer.Close()

	testCases := []struct {
		expectedErr error
		wantAnyErr  bool
		name        string
		url         string
	}{
		{
			expectedErr: nil,
			name:        "existing file",
			url:         server.URL + "/schema-good",
		},
		{
			expectedErr: errors.New(`yaml: unmarshal errors:
  line 3: field requirXXed not found in type model.Variable
  line 6: field pathX not found in type model.Layer
  line 9: field valuXXes not found in type model.Layer`),
			name: "existing file, malformed body",
			url:  server.URL + "/schema-malformed",
		},
		{
			expectedErr: errors.New(`EOF`),
			name:        "existing file, malformed body",
			url:         server.URL + "/schema-empty",
		},
		{
			expectedErr: errors.New(`unexpected status: 404 Not Found`),
			name:        "non-existing file",
			url:         server.URL + "/schema-missing",
		},
		{
			expectedErr: errors.New(`unexpected status: 503 Service Unavailable`),
			name:        "503 error on getting a file",
			url:         server.URL + "/schema-service-unavailable",
		},
		{
			wantAnyErr: true,
			name:       "transport error returns error",
			url:        closedServer.URL + "/schema-good",
		},
	}

	rc := KonfigurationReconciler{}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d: %s", i, tc.name), func(t *testing.T) {
			file, err := rc.fetchKonfigurationSchemaFromUrl(context.Background(), "testing", tc.url)

			if err != nil {
				if tc.expectedErr == nil && !tc.wantAnyErr {
					t.Fatalf("unexpected error on fetching %s schema: %v", tc.url, err)
				}

				if tc.expectedErr != nil && err.Error() != tc.expectedErr.Error() {
					t.Fatalf("error does not match, expected: %v, got: %v", tc.expectedErr, err)
				}
			} else if tc.expectedErr != nil || tc.wantAnyErr {
				t.Fatalf("expected error but got none")
			}

			if tc.expectedErr == nil && !tc.wantAnyErr {
				file = filepath.Clean(file)
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatalf("unexpected error on reading %s file: %v", file, err)
				}

				if string(data) != konfSchema {
					t.Fatalf("schemas mismatch \n %s", cmp.Diff(konfSchema, string(data)))
				}
			}
		})
	}
}
