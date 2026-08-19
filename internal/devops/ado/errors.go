// Package ado is the shared Azure DevOps REST client foundation used by
// internal/devops, internal/devops/repos, internal/devops/pipelines and
// internal/devops/boards.
package ado

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// APIError is a non-2xx response from Azure DevOps.
type APIError struct {
	Status  int    // HTTP status code
	Message string // extracted or raw message
	TypeKey string // WrappedException.typeKey, e.g. "ProjectDoesNotExistWithNameException"; "" if absent
}

func (e *APIError) Error() string { return e.Message }

// newAPIError builds an APIError from a non-2xx response, ported from
// devops_sdk/client.py:240-272 (_handle_error).
func newAPIError(status int, contentType string, body []byte, reqURL string) *APIError {
	body = bytes.TrimSpace(body)

	// client.py:243: a missing Content-Type goes down the JSON path too —
	// only an explicit "text/plain" Content-Type takes the raw-body branch.
	if contentType != "" && strings.Contains(contentType, "text/plain") {
		// client.py:263-264: response.content is never nil here, so the
		// two-space separator is always appended, even for an empty body.
		message := string(body) + "  "
		return &APIError{Status: status, Message: finish(status, message, reqURL)}
	}

	message, typeKey := parseJSONError(body)
	if message != "" {
		// client.py:245-260: a matched WrappedException/ImproperException/
		// SystemException raises immediately with just its own message
		// (exceptions.py:29-41) — no "Operation returned..." suffix and no
		// 401 phrase, even when status is 401.
		return &APIError{Status: status, Message: message, TypeKey: typeKey}
	}

	// Nothing parsed: client.py falls through to the bottom format with an
	// empty error_message.
	return &APIError{Status: status, Message: finish(status, "", reqURL)}
}

// finish applies client.py:265-272's bottom formatting: message is prefixed
// (already containing any two-space separator) onto the 401 phrase or the
// generic status-code sentence.
func finish(status int, message, reqURL string) string {
	if status == 401 {
		return message + "The requested resource requires user authentication: " + reqURL
	}
	return message + fmt.Sprintf("Operation returned a %d status code.", status)
}

// parseJSONError tries, in order, the three JSON error shapes Azure DevOps
// returns (_models.py:162-187 WrappedException, ImproperException,
// SystemException), returning the first non-empty message found.
func parseJSONError(body []byte) (message, typeKey string) {
	// WrappedException: the normal, controlled API error.
	var we struct {
		Message string `json:"message"`
		TypeKey string `json:"typeKey"`
	}
	if err := json.Unmarshal(body, &we); err == nil && we.Message != "" {
		return we.Message, we.TypeKey
	}

	// ImproperException: {"Message": "..."} (PascalCase, ASP.NET-level),
	// tried only when the body is a {"count":N,"value":[...]} collection
	// wrapper (devops_sdk/client.py:252-257) — Python never tries it
	// against a raw, non-collection body.
	type messageOnly struct {
		Message string `json:"Message"`
	}
	var coll struct {
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(body, &coll); err == nil && len(coll.Value) > 0 {
		var ie messageOnly
		if err := json.Unmarshal(coll.Value, &ie); err == nil && ie.Message != "" {
			return ie.Message, ""
		}
	}

	// SystemException: raw .NET serialization, same Message field, tried
	// against the raw body.
	var se messageOnly
	if err := json.Unmarshal(body, &se); err == nil && se.Message != "" {
		return se.Message, ""
	}

	return "", ""
}
