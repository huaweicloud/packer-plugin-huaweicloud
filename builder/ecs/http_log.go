package ecs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
)

func logRequestHandler(request http.Request) {
	log.Printf("[DEBUG] API Request URL: %s %s", request.Method, request.URL)
	log.Printf("[DEBUG] API Request Headers:\n%s", FormatHeaders(request.Header, "\n"))
	if request.Body != nil {
		if err := logRequest(request.Body, request.Header.Get("Content-Type")); err != nil {
			log.Printf("[WARN] failed to get request body: %s", err)
		}
	}
}

func logResponseHandler(response http.Response) {
	log.Printf("[DEBUG] API Response Code: %d", response.StatusCode)
	log.Printf("[DEBUG] API Response Headers:\n%s", FormatHeaders(response.Header, "\n"))

	if err := logResponse(response.Body, response.Header.Get("Content-Type")); err != nil {
		log.Printf("[WARN] failed to get response body: %s", err)
	}
}

// logRequest will log the HTTP Request details.
// If the body is JSON, it will attempt to be pretty-formatted.
func logRequest(original io.ReadCloser, contentType string) error {
	defer original.Close()

	var bs bytes.Buffer
	_, err := io.Copy(&bs, original)
	if err != nil {
		return err
	}

	body := bs.Bytes()
	index := findJSONIndex(body)
	if index == -1 {
		return nil
	}

	// Handle request contentType
	if strings.HasPrefix(contentType, "application/json") {
		debugInfo := formatJSON(body[index:])
		log.Printf("[DEBUG] API Request Body: %s", debugInfo)
	} else {
		log.Printf("[DEBUG] Not logging because the request body isn't JSON")
	}

	return nil
}

// logResponse will log the HTTP Response details.
// If the body is JSON, it will attempt to be pretty-formatted.
func logResponse(original io.ReadCloser, contentType string) error {
	defer original.Close()

	var bs bytes.Buffer
	_, err := io.Copy(&bs, original)
	if err != nil {
		return err
	}

	body := bs.Bytes()
	index := findJSONIndex(body)
	if index == -1 {
		return nil
	}

	if strings.HasPrefix(contentType, "application/json") {
		debugInfo := formatJSON(body[index:])
		log.Printf("[DEBUG] API Response Body: %s", debugInfo)
	} else {
		log.Printf("[DEBUG] Not logging because the response body isn't JSON")
	}

	return nil
}

// FormatHeaders processes a headers object plus a deliminator, returning a string
func FormatHeaders(headers http.Header, seperator string) string {
	redactedHeaders := redactHeaders(headers)
	sort.Strings(redactedHeaders)

	return strings.Join(redactedHeaders, seperator)
}

// redactHeaders processes a headers object, returning a redacted list.
func redactHeaders(headers http.Header) (processedHeaders []string) {
	// sensitiveWords is a list of headers that need to be redacted.
	var sensitiveWords = []string{"token", "authorization"}

	for name, header := range headers {
		for _, v := range header {
			if isSliceContainsStr(sensitiveWords, name) {
				processedHeaders = append(processedHeaders, fmt.Sprintf("%v: %v", name, "***"))
			} else {
				processedHeaders = append(processedHeaders, fmt.Sprintf("%v: %v", name, v))
			}
		}
	}
	return
}

// formatJSON will try to pretty-format a JSON body.
// It will also mask known fields which contain sensitive information.
func formatJSON(raw []byte) string {
	var data map[string]interface{}

	if len(raw) == 0 {
		return ""
	}

	err := json.Unmarshal(raw, &data)
	if err != nil {
		log.Printf("[DEBUG] Unable to parse JSON: %s", err)
		return string(raw)
	}

	// Mask known password fields
	if v, ok := data["auth"].(map[string]interface{}); ok {
		if v, ok := v["identity"].(map[string]interface{}); ok {
			if v, ok := v["password"].(map[string]interface{}); ok {
				if v, ok := v["user"].(map[string]interface{}); ok {
					v["password"] = "***"
				}
			}
		}
	}

	// Ignore the catalog
	if _, ok := data["catalog"].([]interface{}); ok {
		return "{ **skipped** }"
	}
	if v, ok := data["token"].(map[string]interface{}); ok {
		if _, ok := v["catalog"]; ok {
			return "{ **skipped** }"
		}
	}

	pretty, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Printf("[DEBUG] Unable to re-marshal JSON: %s", err)
		return string(raw)
	}

	return string(pretty)
}

func findJSONIndex(raw []byte) int {
	var index = -1
	for i, v := range raw {
		if v == '{' {
			index = i
			break
		}
	}

	return index
}

func isSliceContainsStr(array []string, str string) bool {
	str = strings.ToLower(str)
	for _, s := range array {
		s = strings.ToLower(s)
		if strings.Contains(str, s) {
			return true
		}
	}
	return false
}
