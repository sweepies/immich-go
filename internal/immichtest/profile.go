// Package immichtest provides deterministic HTTP contract fixtures for the
// supported Immich server generations.
package immichtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"strings"
	"sync"
	"testing"
)

const defaultPageSize = 100

type errorGeneration uint8

const (
	legacyErrors errorGeneration = iota
	zodErrors
)

// Profile describes one supported Immich server contract.
type Profile struct {
	Version        string
	APIKey         string
	UserID         string
	UserEmail      string
	SupportedMedia map[string][]string
	PageSize       int

	errorGeneration errorGeneration
}

// V275 returns an isolated Immich v2.7.5 contract profile.
func V275() Profile {
	return Profile{
		Version:   "v2.7.5",
		APIKey:    "contract-api-key",
		UserID:    "00000000-0000-4000-8000-000000000275",
		UserEmail: "v2@example.test",
		SupportedMedia: map[string][]string{
			"image": {".jpg", ".png"},
			"video": {".mp4"},
		},
		PageSize:        defaultPageSize,
		errorGeneration: legacyErrors,
	}
}

// V310 returns an isolated Immich v3.1.0 contract profile.
func V310() Profile {
	return Profile{
		Version:   "v3.1.0",
		APIKey:    "contract-api-key",
		UserID:    "00000000-0000-4000-8000-000000000310",
		UserEmail: "v3@example.test",
		SupportedMedia: map[string][]string{
			"image": {".jpg", ".png"},
			"video": {".mp4"},
		},
		PageSize:        defaultPageSize,
		errorGeneration: zodErrors,
	}
}

func (profile Profile) clone() Profile {
	clone := profile
	clone.SupportedMedia = make(map[string][]string, len(profile.SupportedMedia))
	for mediaType, extensions := range profile.SupportedMedia {
		clone.SupportedMedia[mediaType] = append([]string(nil), extensions...)
	}
	if clone.PageSize <= 0 {
		clone.PageSize = defaultPageSize
	}
	return clone
}

// ValidationError is a field-level validation failure used by both profiles.
type ValidationError struct {
	Path    []any  `json:"path"`
	Message string `json:"message"`
}

// Response is a deterministic response returned by a custom route.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// JSONResponse creates a response containing the JSON-encoded value.
// It panics if the value cannot be encoded as JSON.
func JSONResponse(status int, value any) Response {
	body, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("encode contract response: %v", err))
	}
	return Response{
		Status: status,
		Header: http.Header{"Content-Type": {"application/json"}},
		Body:   body,
	}
}

// ValidationResponse emits the validation-error shape for this profile.
func (profile Profile) ValidationResponse(status int, correlationID string, validationErrors ...ValidationError) Response {
	if profile.errorGeneration == zodErrors {
		response := JSONResponse(status, struct {
			Message string            `json:"message"`
			Errors  []ValidationError `json:"errors"`
		}{Message: "Validation failed", Errors: validationErrors})
		if correlationID != "" {
			response.Header.Set("X-Correlation-ID", correlationID)
		}
		return response
	}

	messages := make([]string, 0, len(validationErrors))
	for _, validationError := range validationErrors {
		messages = append(messages, validationError.Message)
	}
	return JSONResponse(status, struct {
		Message       []string `json:"message"`
		Error         string   `json:"error"`
		StatusCode    int      `json:"statusCode"`
		CorrelationID string   `json:"correlationId,omitempty"`
	}{
		Message:       messages,
		Error:         http.StatusText(status),
		StatusCode:    status,
		CorrelationID: correlationID,
	})
}

// MultipartField captures one multipart form field or file.
type MultipartField struct {
	Filename string
	Header   textproto.MIMEHeader
	Value    []byte
}

// Request is an immutable snapshot of one request received by a Server.
type Request struct {
	Method    string
	Path      string
	Query     url.Values
	Header    http.Header
	Body      []byte
	JSON      json.RawMessage
	Multipart map[string][]MultipartField
}

// captureRequest snapshots request metadata and body content, parsing JSON and multipart
// form data into the corresponding Request fields. It returns an error if the body
// cannot be read or multipart data cannot be parsed.
func captureRequest(request *http.Request) (Request, error) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return Request{}, err
	}
	captured := Request{
		Method: request.Method,
		Path:   request.URL.Path,
		Query:  request.URL.Query(),
		Header: request.Header.Clone(),
		Body:   append([]byte(nil), body...),
	}
	mediaType, parameters, _ := mime.ParseMediaType(request.Header.Get("Content-Type"))
	switch mediaType {
	case "application/json":
		captured.JSON = append(json.RawMessage(nil), body...)
	case "multipart/form-data":
		reader := multipart.NewReader(bytes.NewReader(body), parameters["boundary"])
		captured.Multipart = make(map[string][]MultipartField)
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return Request{}, err
			}
			value, err := io.ReadAll(part)
			if err != nil {
				return Request{}, err
			}
			field := MultipartField{
				Filename: part.FileName(),
				Header:   textproto.MIMEHeader(http.Header(part.Header).Clone()),
				Value:    value,
			}
			captured.Multipart[part.FormName()] = append(captured.Multipart[part.FormName()], field)
		}
	}
	return captured, nil
}

func cloneValues(values url.Values) url.Values {
	clone := make(url.Values, len(values))
	for name, entries := range values {
		clone[name] = append([]string(nil), entries...)
	}
	return clone
}

func (request Request) clone() Request {
	clone := request
	clone.Query = cloneValues(request.Query)
	clone.Header = request.Header.Clone()
	clone.Body = append([]byte(nil), request.Body...)
	clone.JSON = append(json.RawMessage(nil), request.JSON...)
	if request.Multipart != nil {
		clone.Multipart = make(map[string][]MultipartField, len(request.Multipart))
		for name, fields := range request.Multipart {
			fieldClones := make([]MultipartField, len(fields))
			for i, field := range fields {
				fieldClones[i] = MultipartField{
					Filename: field.Filename,
					Header:   textproto.MIMEHeader(http.Header(field.Header).Clone()),
					Value:    append([]byte(nil), field.Value...),
				}
			}
			clone.Multipart[name] = fieldClones
		}
	}
	return clone
}

// Handler handles a captured custom-route request.
type Handler func(Request) Response

// Server is an isolated httptest server backed by one contract profile.
type Server struct {
	*httptest.Server

	profile  Profile
	mu       sync.RWMutex
	routes   map[string]Handler
	requests []Request
}

// NewServer starts a profile server and registers cleanup with the test.
func NewServer(t testing.TB, profile Profile) *Server {
	t.Helper()
	server := &Server{
		profile: profile.clone(),
		routes:  make(map[string]Handler),
	}
	server.Server = httptest.NewServer(http.HandlerFunc(server.serveHTTP))
	t.Cleanup(server.Close)
	return server
}

// Profile returns an isolated copy of the server's profile.
func (server *Server) Profile() Profile {
	return server.profile.clone()
}

// Handle registers or replaces an exact method and path handler.
func (server *Server) Handle(method, path string, handler Handler) {
	server.mu.Lock()
	defer server.mu.Unlock()
	server.routes[routeKey(method, path)] = handler
}

// Requests returns immutable snapshots in receive order.
func (server *Server) Requests() []Request {
	server.mu.RLock()
	defer server.mu.RUnlock()
	requests := make([]Request, len(server.requests))
	for i, request := range server.requests {
		requests[i] = request.clone()
	}
	return requests
}

func (server *Server) serveHTTP(response http.ResponseWriter, request *http.Request) {
	captured, err := captureRequest(request)
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}
	server.mu.Lock()
	server.requests = append(server.requests, captured.clone())
	handler := server.routes[routeKey(request.Method, request.URL.Path)]
	server.mu.Unlock()
	if handler != nil {
		writeResponse(response, handler(captured))
		return
	}
	writeResponse(response, server.builtinResponse(captured))
}

func (server *Server) builtinResponse(request Request) Response {
	if request.Method != http.MethodGet {
		return server.profile.ValidationResponse(http.StatusMethodNotAllowed, "method-not-allowed", ValidationError{Message: "Method not allowed"})
	}
	if request.Header.Get("x-api-key") != server.profile.APIKey {
		return server.profile.ValidationResponse(http.StatusUnauthorized, "unauthorized", ValidationError{Message: "Invalid API key"})
	}
	switch request.Path {
	case "/api/server/ping":
		return JSONResponse(http.StatusOK, map[string]string{"res": "pong"})
	case "/api/users/me":
		return JSONResponse(http.StatusOK, map[string]string{
			"id":    server.profile.UserID,
			"email": server.profile.UserEmail,
			"name":  strings.TrimSuffix(server.profile.UserEmail, "@example.test"),
		})
	case "/api/server/about":
		return JSONResponse(http.StatusOK, map[string]any{"version": server.profile.Version, "licensed": true})
	case "/api/server/media-types":
		return JSONResponse(http.StatusOK, server.profile.SupportedMedia)
	default:
		return server.profile.ValidationResponse(http.StatusNotFound, "not-found", ValidationError{Message: "Not found"})
	}
}

func routeKey(method, path string) string {
	return method + " " + path
}

// writeResponse writes a response's headers, status, and body to the provided writer.
// An unset status is written as HTTP 200 OK.
func writeResponse(writer http.ResponseWriter, response Response) {
	for name, values := range response.Header {
		for _, value := range values {
			writer.Header().Add(name, value)
		}
	}
	status := response.Status
	if status == 0 {
		status = http.StatusOK
	}
	writer.WriteHeader(status)
	_, _ = writer.Write(response.Body)
}

// Paginate returns a one-based page of items using the profile's page size.
// Invalid page sizes use the default, and page numbers less than one use the
// first page. It reports whether more items remain after the returned page.
func Paginate[T any](profile Profile, items []T, page int) (values []T, hasNext bool) {
	pageSize := profile.PageSize
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if page < 1 {
		page = 1
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []T{}, false
	}
	end := min(start+pageSize, len(items))
	return append([]T(nil), items[start:end]...), end < len(items)
}
