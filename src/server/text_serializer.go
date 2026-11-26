package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	commonhttputil "github.com/apimgr/airports/src/common/httputil"
)

type capturedResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (response *capturedResponse) Header() http.Header { return response.header }
func (response *capturedResponse) WriteHeader(status int) {
	if response.status == 0 {
		response.status = status
	}
}

func (response *capturedResponse) Write(data []byte) (int, error) {
	if response.status == 0 {
		response.status = http.StatusOK
	}
	return response.body.Write(data)
}

func textNegotiatedHandler(route Route, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if route.Binary != "" || commonhttputil.GetAPIResponseFormat(r) != "text" {
			next(w, r)
			return
		}
		captured := &capturedResponse{header: make(http.Header)}
		next(captured, r)
		for key, values := range captured.header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		if captured.status == 0 {
			captured.status = http.StatusOK
		}
		contentType := captured.header.Get("Content-Type")
		if captured.status >= 400 || !strings.Contains(contentType, "application/json") {
			w.WriteHeader(captured.status)
			_, _ = w.Write(captured.body.Bytes())
			return
		}
		var payload any
		if err := json.Unmarshal(captured.body.Bytes(), &payload); err != nil {
			w.WriteHeader(captured.status)
			_, _ = w.Write(captured.body.Bytes())
			return
		}
		text, ok := serializeRouteResponse(route, r.Method, payload)
		if !ok {
			w.WriteHeader(captured.status)
			_, _ = w.Write(captured.body.Bytes())
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(captured.status)
		_, _ = io.WriteString(w, strings.TrimRight(text, "\n")+"\n")
	}
}

func serializeRouteResponse(route Route, method string, payload any) (string, bool) {
	root, ok := payload.(map[string]any)
	if !ok {
		return serializeTextValue(payload), true
	}
	if root["ok"] == false {
		code, _ := root["error"].(string)
		message, _ := root["message"].(string)
		return "ERROR: " + code + ": " + message, true
	}
	data, hasData := root["data"]
	if !hasData {
		data = root
	}
	if route.Primary != "" {
		if object, isObject := data.(map[string]any); isObject {
			if value, found := object[route.Primary]; found {
				return serializeTextValue(value), true
			}
		}
		return "", false
	}
	message, _ := root["message"].(string)
	if message == "" {
		message = route.Message
	}
	if message == "" {
		message = operationDefaultMessage(method, route.Path)
	}
	if !hasData || data == nil {
		return "OK: " + message, true
	}
	if values, isList := data.([]any); isList {
		var lines []string
		lines = append(lines, fmt.Sprintf("OK: %d items found", len(values)))
		for index, value := range values {
			if index > 0 {
				if _, object := value.(map[string]any); object {
					lines = append(lines, "")
				}
			}
			if object, isObject := value.(map[string]any); isObject {
				lines = append(lines, formatObject(object, route.Response, "")...)
			} else {
				lines = append(lines, serializeTextValue(value))
			}
		}
		return strings.Join(lines, "\n"), true
	}
	if object, isObject := data.(map[string]any); isObject {
		return strings.Join(append([]string{"OK: " + message}, formatObject(object, route.Response, "")...), "\n"), true
	}
	return "OK: " + message + "\n" + serializeTextValue(data), true
}

func operationDefaultMessage(method, path string) string {
	switch method {
	case http.MethodGet:
		return "Retrieved successfully"
	case http.MethodPost:
		if strings.HasSuffix(path, "/server/preferences/tokens") || strings.HasSuffix(path, "/server/preferences/tokens/primary") || strings.HasSuffix(path, "/server/contact") {
			return "Created successfully"
		}
		if strings.HasPrefix(path, "/api/") && strings.Contains(path, "/preferences") {
			return "Updated successfully"
		}
		return "OK"
	case http.MethodPut, http.MethodPatch:
		return "Updated successfully"
	case http.MethodDelete:
		return "Deleted successfully"
	default:
		return "OK"
	}
}

func formatObject(object map[string]any, responseModel any, prefix string) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	order := responseFieldOrder(responseModel)
	positions := make(map[string]int, len(order))
	for index, key := range order {
		positions[key] = index
	}
	sort.SliceStable(keys, func(i, j int) bool {
		left, leftOK := positions[keys[i]]
		right, rightOK := positions[keys[j]]
		if leftOK != rightOK {
			return leftOK
		}
		if leftOK && left != right {
			return left < right
		}
		return keys[i] < keys[j]
	})
	var lines []string
	for _, key := range keys {
		value := object[key]
		field := key
		if prefix != "" {
			field = prefix + "." + key
		}
		if nested, ok := value.(map[string]any); ok {
			lines = append(lines, formatObject(nested, nil, field)...)
			continue
		}
		lines = append(lines, field+": "+serializeTextValue(value))
	}
	return lines
}

func responseFieldOrder(response any) []string {
	operation, ok := response.(map[string]any)
	if !ok {
		return nil
	}
	responses, _ := operation["200"].(map[string]any)
	content, _ := responses["content"].(map[string]any)
	for _, media := range content {
		entry, _ := media.(map[string]any)
		schema, _ := entry["schema"].(map[string]any)
		properties, _ := schema["properties"].(map[string]any)
		if len(properties) > 0 {
			keys := make([]string, 0, len(properties))
			for key := range properties {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			return keys
		}
	}
	return nil
}

func serializeTextValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case []any:
		values := make([]string, len(typed))
		for index, item := range typed {
			values[index] = serializeTextValue(item)
		}
		return strings.Join(values, ", ")
	case map[string]any:
		return fmt.Sprint(typed)
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}
