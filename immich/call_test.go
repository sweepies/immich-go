package immich

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sweepies/immich-go/internal/immichtest"
)

type testServer struct {
	// endpoint       string
	responseStatus int
	responseBody   string
}

func (ts *testServer) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	resp.WriteHeader(ts.responseStatus)
	_, _ = resp.Write([]byte(ts.responseBody))
}

func TestCall(t *testing.T) {
	tt := []struct {
		name        string
		requestFn   requestFunction
		expectedErr bool
		server      testServer
	}{
		{
			name:        "happy path",
			requestFn:   getRequest("/assets", setAcceptJSON()),
			expectedErr: false,
			server: testServer{
				responseStatus: http.StatusOK,
				responseBody:   `{"status": "All correct"}`,
			},
		},
		{
			name:        "bad url",
			requestFn:   getRequest("/ass\nets", setAcceptJSON()),
			expectedErr: true,
			server: testServer{
				responseStatus: http.StatusOK,
				responseBody:   `{"status": "All correct"}`,
			},
		},
		{
			name:        "post / ok",
			requestFn:   postRequest("/albums", "application/json", setAcceptJSON(), setJSONBody(struct{ Name string }{Name: "test"})),
			expectedErr: false,
			server: testServer{
				responseStatus: http.StatusOK,
				responseBody:   `{"Name": "test"}`,
			},
		},
		{
			name:        "bad request / post",
			requestFn:   postRequest("/albums", "application/json", setAcceptJSON(), setJSONBody(struct{ Name string }{Name: "test"})),
			expectedErr: true,
			server: testServer{
				responseStatus: http.StatusBadRequest,
				responseBody:   `{"error": "Bad request", "statusCode": "400", "message": ["String1","String2"]}`,
			},
		},
	}

	for _, tst := range tt {
		t.Run(tst.name, func(t *testing.T) {
			server := httptest.NewServer(&tst.server)
			defer server.Close()
			ctx := context.Background()
			ic, err := NewImmichClient(server.URL, "1234")
			if err != nil {
				t.Fail()
				return
			}
			// ic.EnableAppTrace(true)
			r := map[string]string{}
			err = ic.newServerCall(ctx, tst.name).do(tst.requestFn, responseJSON(&r))
			if tst.expectedErr && err == nil {
				t.Errorf("expected error, but no error")
			}
			if !tst.expectedErr && err != nil {
				t.Errorf("no error expected, but error: %s", err.Error())
			}
			if err != nil {
				t.Logf("error received: %s", err.Error())
			}
			t.Logf("response received: %#v", r)
		})
	}
}

func TestServerErrorResponseShapes(t *testing.T) {
	tests := []struct {
		name          string
		body          string
		correlationID string
		expected      []string
	}{
		{
			name: "legacy validation response",
			body: `{
				"message": ["[comment] must not be provided", "[type] is invalid"],
				"error": "Bad Request",
				"statusCode": 400,
				"correlationId": "legacy-correlation"
			}`,
			expected: []string{
				"[comment] must not be provided",
				"[type] is invalid",
				"Correlation ID: legacy-correlation",
			},
		},
		{
			name: "v3 validation response before version discovery",
			body: `{
				"message": "Validation failed",
				"errors": [
					{"path": ["asset", "duration"], "message": "Expected number, received string"},
					{"path": ["comment"], "message": "Comment is required"}
				]
			}`,
			correlationID: "v3-correlation",
			expected: []string{
				"Validation failed",
				`["asset","duration"]: Expected number, received string`,
				`["comment"]: Comment is required`,
				"Correlation ID: v3-correlation",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				response.Header().Set("Content-Type", "application/json")
				if test.correlationID != "" {
					response.Header().Set("X-Correlation-ID", test.correlationID)
				}
				response.WriteHeader(http.StatusBadRequest)
				_, _ = response.Write([]byte(test.body))
			}))
			defer server.Close()

			client, err := NewImmichClient(server.URL, "test-key")
			if err != nil {
				t.Fatal(err)
			}
			err = client.newServerCall(context.Background(), "validation").do(
				postRequest("/albums", "application/json", setAcceptJSON()),
			)
			if err == nil {
				t.Fatal("expected validation error")
			}
			for _, expected := range test.expected {
				if !strings.Contains(err.Error(), expected) {
					t.Errorf("error %q does not contain %q", err, expected)
				}
			}

			callErr, ok := err.(callError)
			if !ok {
				t.Fatalf("expected callError, got %T", err)
			}
			if callErr.status != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", callErr.status, http.StatusBadRequest)
			}
			if callErr.method != http.MethodPost {
				t.Errorf("method = %q, want %q", callErr.method, http.MethodPost)
			}
			if !strings.HasSuffix(callErr.url, "/api/albums") {
				t.Errorf("url = %q, want suffix /api/albums", callErr.url)
			}
		})
	}
}

type errorRoundTripper func(*http.Request) (*http.Response, error)

func (roundTripper errorRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTripper(request)
}

func TestCallPreservesTransportErrorContext(t *testing.T) {
	transportErr := errors.New("transport failed")
	client, err := NewImmichClient("http://immich.example", "test-key")
	if err != nil {
		t.Fatal(err)
	}
	client.client.Transport = errorRoundTripper(func(*http.Request) (*http.Response, error) {
		return nil, transportErr
	})

	err = client.newServerCall(context.Background(), "transport").do(getRequest("/assets"))
	if err == nil {
		t.Fatal("expected transport error")
	}
	callErr, ok := err.(callError)
	if !ok {
		t.Fatalf("expected callError, got %T", err)
	}
	if !errors.Is(callErr.err, transportErr) {
		t.Fatalf("wrapped transport error = %v, want %v", callErr.err, transportErr)
	}
	for _, expected := range []string{"transport", http.MethodGet, "http://immich.example/api/assets", transportErr.Error()} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("error %q does not contain %q", err, expected)
		}
	}
}

func TestProfileErrorContracts(t *testing.T) {
	for _, profile := range []immichtest.Profile{immichtest.V275(), immichtest.V310()} {
		t.Run(profile.Version, func(t *testing.T) {
			server := immichtest.NewServer(t, profile)
			server.Handle(http.MethodGet, "/api/assets/missing-id", func(request immichtest.Request) immichtest.Response {
				return profile.ValidationResponse(http.StatusBadRequest, "profile-correlation", immichtest.ValidationError{
					Path:    []any{"id"},
					Message: "Invalid asset ID",
				})
			})
			client, err := NewImmichClient(server.URL, profile.APIKey)
			if err != nil {
				t.Fatal(err)
			}

			_, err = client.GetAssetInfo(context.Background(), "missing-id")
			if err == nil {
				t.Fatal("expected asset validation error")
			}
			for _, expected := range []string{
				"400 Bad Request",
				http.MethodGet,
				"/api/assets/missing-id",
				"Invalid asset ID",
				"Correlation ID: profile-correlation",
			} {
				if !strings.Contains(err.Error(), expected) {
					t.Errorf("error %q does not contain %q", err, expected)
				}
			}
			if profile.Version == "v3.1.0" && !strings.Contains(err.Error(), `["id"]: Invalid asset ID`) {
				t.Errorf("v3 error does not preserve nested path: %q", err)
			}
			if client.ServerVersion().String() != "" {
				t.Errorf("error handling unexpectedly discovered version %q", client.ServerVersion())
			}
		})
	}
}
