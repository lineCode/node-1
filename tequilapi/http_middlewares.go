package tequilapi

import "net/http"

type corsHandler struct {
	originalHandler http.Handler
}

func (wrapper corsHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	if isPreflightCorsRequest(req) {
		applyPreflightCorsResponse(resp)
		return
	}

	allowAllCorsActions(resp)
	wrapper.originalHandler.ServeHTTP(resp, req)
}

//ApplyCors wraps original handler by adding cors headers to response BEFORE original ServeHTTP method is called
func ApplyCors(original http.Handler) http.Handler {
	return corsHandler{original}
}

func allowAllCorsActions(resp http.ResponseWriter) {
	resp.Header().Set("Access-Control-Allow-Origin", "*")
	resp.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	resp.Header().Set("Access-Control-Allow-Headers", "Content-type")
}

func isPreflightCorsRequest(req *http.Request) bool {
	isOptionsMethod := req.Method == http.MethodOptions
	containsAccessControlRequestMethod := req.Header.Get("Access-Control-Request-Method") != ""
	containsAccessControlRequestHeader := req.Header.Get("Access-Control-Request-Headers") != ""
	return isOptionsMethod && containsAccessControlRequestHeader && containsAccessControlRequestMethod
}

func applyPreflightCorsResponse(resp http.ResponseWriter) {
	allowAllCorsActions(resp)
}