package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/apimgr/airports/src/swagger"
	"github.com/go-chi/chi/v5"
)

const currentMCPVersion = "2026-07-28"

type mcpRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      json.RawMessage        `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params"`
}

type mcpTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Annotations map[string]bool        `json:"annotations,omitempty"`
	method      string
	path        string
	body        bool
}

func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	if !s.validMCPOrigin(r) {
		writeMCPHTTPError(w, http.StatusForbidden, "ORIGIN_NOT_ALLOWED", "The request origin is not allowed.")
		return
	}
	if !s.validAPIBearer(r) {
		if s.enforceOwnerTokenAuthFailure(w, r) {
			return
		}
		s.logRejectedCLIToken(r)
		w.Header().Set("WWW-Authenticate", `Bearer`)
		writeMCPRPCError(w, http.StatusUnauthorized, nil, -32001, "A valid Bearer token is required.")
		return
	}
	if r.Method != http.MethodPost {
		writeMCPHTTPError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "MCP accepts POST requests only.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var request mcpRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.JSONRPC != "2.0" || request.Method == "" {
		writeMCPHTTPError(w, http.StatusBadRequest, "INVALID_JSON_RPC", "The request is not a valid JSON-RPC message.")
		return
	}
	version := strings.TrimSpace(r.Header.Get("MCP-Protocol-Version"))
	meta := mcpProtocolVersion(request)
	if (request.Method == "initialize" && meta == "" && version == "") || (version != "" && !supportedMCPVersion(version)) {
		writeMCPRPCError(w, http.StatusBadRequest, request.ID, -32002, "Unsupported protocol version. Supported protocol version: 2026-07-28.")
		return
	}
	if err := s.validateMCPHeaders(r, request); err != nil {
		writeMCPRPCError(w, http.StatusBadRequest, request.ID, -32020, "HeaderMismatch: "+err.Error())
		return
	}
	version = currentMCPVersion
	result, rpcCode, rpcMessage := s.dispatchMCP(r, request, version)
	if rpcCode != 0 {
		status := http.StatusOK
		if rpcCode == -32601 {
			status = http.StatusNotFound
		}
		writeMCPRPCError(w, status, request.ID, rpcCode, rpcMessage)
		return
	}
	if len(request.ID) == 0 || string(request.ID) == "null" {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	writeMCPResult(w, request.ID, result)
}

func (s *Server) validateMCPHeaders(r *http.Request, request mcpRequest) error {
	if value := strings.TrimSpace(r.Header.Get("Mcp-Method")); value == "" || value != request.Method {
		return fmt.Errorf("Mcp-Method does not match the JSON-RPC method")
	}
	if request.Method == "tools/call" || request.Method == "resources/read" || request.Method == "prompts/get" {
		name, _ := request.Params["name"].(string)
		if request.Method == "resources/read" {
			name, _ = request.Params["uri"].(string)
		}
		value := strings.TrimSpace(r.Header.Get("Mcp-Name"))
		if decoded, err := new(mime.WordDecoder).DecodeHeader(value); err == nil {
			value = decoded
		}
		if value == "" || value != name {
			return fmt.Errorf("Mcp-Name does not match the JSON-RPC name")
		}
	}
	version := strings.TrimSpace(r.Header.Get("MCP-Protocol-Version"))
	if version == "" || mcpProtocolVersion(request) != version {
		return fmt.Errorf("MCP-Protocol-Version does not match the JSON-RPC metadata")
	}
	return nil
}

func mcpProtocolVersion(request mcpRequest) string {
	meta, _ := request.Params["_meta"].(map[string]interface{})
	version, _ := meta["io.modelcontextprotocol/protocolVersion"].(string)
	return strings.TrimSpace(version)
}

func supportedMCPVersion(version string) bool {
	return version == currentMCPVersion
}

func (s *Server) dispatchMCP(r *http.Request, request mcpRequest, version string) (interface{}, int, string) {
	switch request.Method {
	case "server/discover":
		return map[string]interface{}{
			"supportedProtocolVersions": []string{currentMCPVersion},
			"capabilities":              map[string]interface{}{"tools": map[string]interface{}{}},
			"_meta":                     map[string]interface{}{"io.modelcontextprotocol/serverInfo": map[string]string{"name": "airports", "version": Version}},
		}, 0, ""
	case "initialize":
		return map[string]interface{}{
			"protocolVersion": currentMCPVersion,
			"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
			"serverInfo":      map[string]string{"name": "airports", "version": Version},
		}, 0, ""
	case "notifications/initialized":
		return nil, 0, ""
	case "ping":
		return map[string]interface{}{}, 0, ""
	case "tools/list":
		tools := s.mcpToolsForRequest(r)
		for i := range tools {
			tools[i].method = ""
			tools[i].path = ""
		}
		return map[string]interface{}{"tools": tools}, 0, ""
	case "tools/call":
		name, _ := request.Params["name"].(string)
		arguments, _ := request.Params["arguments"].(map[string]interface{})
		for _, tool := range s.mcpToolsForRequest(r) {
			if tool.Name == name {
				return s.callMCPTool(r, tool, arguments)
			}
		}
		return nil, -32602, "Unknown tool."
	default:
		return nil, -32601, "Method not found."
	}
}

func (s *Server) mcpTools() []mcpTool {
	return s.mcpToolsForRequest(nil)
}

func (s *Server) mcpToolsForRequest(request *http.Request) []mcpTool {
	if s.routeRegistry == nil {
		return nil
	}
	apiPrefix := s.APIBasePath()
	apiPrefix = strings.TrimSuffix(apiPrefix, "/")
	openAPIRoutes := s.routeRegistry.MCPRoutes(s.APIBasePath())
	openAPI := swagger.BuildEmbeddedSpecWithRoutes(Version, s.config.Server.APIVersion, "en", openAPIRoutes)
	swagger.FilterMountedRoutes(openAPI, openAPIRoutes)
	routesByOperation := make(map[string]Route)
	for _, route := range s.routeRegistry.Snapshot() {
		if route.OperationID != "" {
			routesByOperation[route.OperationID] = route
		}
	}
	paths, _ := openAPI["paths"].(map[string]interface{})
	tools := make([]mcpTool, 0, len(paths))
	for apiPath, rawPathItem := range paths {
		pathItem, _ := rawPathItem.(map[string]interface{})
		for methodName, rawOperation := range pathItem {
			method := strings.ToUpper(methodName)
			if method != http.MethodGet && method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch && method != http.MethodDelete {
				continue
			}
			if apiPath == "/server/mcp" || strings.HasPrefix(apiPath, "/server/preferences/") || apiPath == "/server/preferences" {
				continue
			}
			operation, _ := rawOperation.(map[string]interface{})
			name, _ := operation["operationId"].(string)
			if name == "" {
				continue
			}
			route, exists := routesByOperation[name]
			if !exists || (route.MountedWhen != "" && !route.Mounted) || !s.mcpRouteAvailable(route, request) {
				continue
			}
			if route.Path != apiPrefix && !strings.HasPrefix(route.Path, apiPrefix+"/") {
				continue
			}
			description, _ := operation["description"].(string)
			if description == "" {
				description, _ = operation["summary"].(string)
			}
			properties := make(map[string]interface{})
			required := make([]string, 0)
			bodyOperation := false
			if parameters, ok := operation["parameters"].([]map[string]interface{}); ok {
				for _, parameter := range parameters {
					location, _ := parameter["in"].(string)
					if location != "path" && location != "query" {
						continue
					}
					parameterName, _ := parameter["name"].(string)
					if parameterName == "" {
						continue
					}
					property, ok := parameter["schema"].(map[string]interface{})
					if !ok {
						property = map[string]interface{}{"type": "string"}
					}
					if parameterDescription, ok := parameter["description"].(string); ok && parameterDescription != "" {
						property["description"] = parameterDescription
					}
					properties[parameterName] = property
					if requiredValue, _ := parameter["required"].(bool); requiredValue {
						required = append(required, parameterName)
					}
				}
			}
			if requestBody, ok := operation["requestBody"].(map[string]interface{}); ok {
				if content, ok := requestBody["content"].(map[string]interface{}); ok {
					if jsonContent, ok := content["application/json"].(map[string]interface{}); ok {
						if bodySchema, ok := jsonContent["schema"].(map[string]interface{}); ok {
							bodyOperation = true
							if bodyProperties, ok := bodySchema["properties"].(map[string]interface{}); ok {
								for name, property := range bodyProperties {
									properties[name] = property
								}
							}
							switch bodyRequired := bodySchema["required"].(type) {
							case []string:
								required = append(required, bodyRequired...)
							case []interface{}:
								for _, value := range bodyRequired {
									if name, ok := value.(string); ok {
										required = append(required, name)
									}
								}
							}
						}
					}
				}
			}
			schema := map[string]interface{}{"type": "object", "properties": properties, "additionalProperties": bodyOperation}
			if len(required) > 0 {
				schema["required"] = required
			}
			annotations := map[string]bool{"readOnlyHint": method == http.MethodGet, "destructiveHint": method == http.MethodDelete, "idempotentHint": method == http.MethodPut || method == http.MethodDelete}
			tools = append(tools, mcpTool{
				Name: name, Description: description, InputSchema: schema,
				Annotations: annotations,
				method:      method, path: apiPath, body: bodyOperation,
			})
		}
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools
}

func (s *Server) mcpRouteAvailable(route Route, request *http.Request) bool {
	if route.Internal || route.Auth == "service" {
		return false
	}
	if request == nil {
		return route.Auth == "none"
	}
	switch route.Auth {
	case "none":
		return true
	case "token":
		return s.validAPIBearer(request)
	case "owner":
		identity, ok := s.ownerTokenFromBearer(request, false)
		return ok && identity.Kind != "operator"
	case "operator":
		identity, ok := s.ownerTokenFromBearer(request, false)
		return ok && identity.Kind == "operator"
	default:
		return false
	}
}

func (s *Server) callMCPTool(r *http.Request, tool mcpTool, arguments map[string]interface{}) (interface{}, int, string) {
	path := tool.path
	query := url.Values{}
	bodyValues := make(map[string]interface{})
	for key, value := range arguments {
		placeholder := "{" + key + "}"
		if strings.Contains(path, placeholder) {
			text, ok := value.(string)
			if !ok || text == "" {
				return nil, -32602, "A required path argument is missing or invalid."
			}
			path = strings.ReplaceAll(path, placeholder, url.PathEscape(text))
			continue
		}
		if tool.body {
			bodyValues[key] = value
			continue
		}
		if text, ok := value.(string); ok {
			query.Set(key, text)
			continue
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, -32602, "An argument could not be encoded."
		}
		query.Set(key, string(encoded))
	}
	if strings.Contains(path, "{") {
		return nil, -32602, "A required path argument is missing."
	}
	target := s.APIBasePath() + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	var body io.Reader
	if tool.body {
		encoded, err := json.Marshal(bodyValues)
		if err != nil {
			return nil, -32602, "Arguments could not be encoded."
		}
		body = bytes.NewReader(encoded)
	}
	request := r.Clone(r.Context())
	request = request.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, chi.NewRouteContext()))
	request.Method = tool.method
	request.URL, _ = url.Parse(target)
	request.RequestURI = target
	request.Body = io.NopCloser(body)
	request.Header = r.Header.Clone()
	request.Header.Del("Accept-Encoding")
	request.Header.Set("Accept", "application/json")
	if tool.body {
		request.Header.Set("Content-Type", "application/json")
	}
	response := newMCPRecorder()
	s.router.ServeHTTP(response, request)
	result := map[string]interface{}{
		"content": []map[string]string{{"type": "text", "text": response.body.String()}},
	}
	if response.status >= 400 {
		result["isError"] = true
	}
	return result, 0, ""
}

type mcpRecorder struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func newMCPRecorder() *mcpRecorder            { return &mcpRecorder{header: make(http.Header)} }
func (w *mcpRecorder) Header() http.Header    { return w.header }
func (w *mcpRecorder) WriteHeader(status int) { w.status = status }
func (w *mcpRecorder) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(body)
}

func (s *Server) validMCPOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	requestOrigin, err := url.Parse(strings.TrimSuffix(RequestBaseURL(r), "/"))
	if err != nil || requestOrigin.Host == "" {
		return false
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	if strings.EqualFold(parsed.Scheme, requestOrigin.Scheme) && strings.EqualFold(parsed.Host, requestOrigin.Host) {
		return true
	}
	for _, allowed := range s.config.Server.CORS.AllowedOrigins {
		if strings.EqualFold(strings.TrimSpace(allowed), origin) {
			return true
		}
	}
	return false
}

func writeMCPHTTPError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": code, "message": message, "details": map[string]interface{}{}})
}

func writeMCPRPCError(w http.ResponseWriter, status int, id json.RawMessage, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": id, "error": map[string]interface{}{"code": code, "message": message}})
}

func writeMCPResult(w http.ResponseWriter, id json.RawMessage, result interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": id, "result": result})
}
