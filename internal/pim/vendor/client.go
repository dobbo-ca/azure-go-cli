/*
Copyright © 2023 netr0m <netr0m@pm.me>
*/
package pimvendor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Azure Client interface
type Client interface {
	GetEligibleResourceAssignments(token string) (*ResourceAssignmentResponse, error)
	GetEligibleGovernanceRoleAssignments(roleType string, subjectId string, token string) (*GovernanceRoleAssignmentResponse, error)
	ValidateResourceAssignmentRequest(scope string, resourceAssignmentRequest *ResourceAssignmentRequestRequest, token string) (bool, error)
	ValidateGovernanceRoleAssignmentRequest(roleType string, roleAssignmentRequest *GovernanceRoleAssignmentRequest, token string) (bool, error)
	RequestResourceAssignment(scope string, resourceAssignmentRequest *ResourceAssignmentRequestRequest, token string) (*ResourceAssignmentRequestResponse, error)
	RequestGovernanceRoleAssignment(roleType string, governanceRoleAssignmentRequest *GovernanceRoleAssignmentRequest, token string) (*GovernanceRoleAssignmentRequestResponse, error)
}

// Azure Client implementation
type AzureClient struct {
	ARMBaseURL string
	ASMScope   string
}

func GetUserInfo(token string) (AzureUserInfo, error) {
	// Decode token
	decoded, err := jwt.ParseWithClaims(token, &AzureUserInfoClaims{}, nil)
	if decoded == nil {
		return AzureUserInfo{}, fmt.Errorf("GetUserInfo: %w", err)
	}

	// Parse claims
	claims, ok := decoded.Claims.(*AzureUserInfoClaims)
	if !ok {
		return AzureUserInfo{}, fmt.Errorf("GetUserInfo: unexpected claims type %T", decoded.Claims)
	}

	return claims.AzureUserInfo, nil
}

func handleRequestErr(_error *Error, err error, req *http.Request) error {
	_error.Message = err.Error()
	_error.Err = err
	_error.Request = req
	return _error
}

func Request(request *PIMRequest, responseModel any) (any, error) {
	// Prepare request body
	var req *http.Request
	var err error
	_error := Error{
		Operation: "Request",
	}

	if request.Payload != nil {
		payload := new(bytes.Buffer)
		json.NewEncoder(payload).Encode(request.Payload) //nolint:errcheck
		req, err = http.NewRequest(request.Method, request.Url, payload)
		if err != nil {
			return nil, handleRequestErr(&_error, err, req)
		}
	} else {
		req, err = http.NewRequest(request.Method, request.Url, nil)
		if err != nil {
			return nil, handleRequestErr(&_error, err, req)
		}
	}

	// Add headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", request.Token))

	// Prepare request parameters
	query := req.URL.Query()
	for k, v := range request.Params {
		query.Add(k, v)
	}
	req.URL.RawQuery = query.Encode()

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		// res is nil on transport errors (connection refused, DNS failure,
		// timeout). Don't touch res fields and don't defer res.Body.Close().
		_error.Message = err.Error()
		_error.Err = err
		_error.Request = req
		slog.Error(_error.Error())
		slog.Debug(_error.Debug())
		return nil, &_error
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			slog.Error(fmt.Sprintf("Failed to close response body: %v", err))
		}
	}()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		_error.Message = err.Error()
		_error.Status = res.Status
		_error.Err = err
		_error.Request = req
		_error.Response = res
		slog.Error(_error.Error())
		slog.Debug(_error.Debug())
		return nil, &_error
	}

	// Handle upstream error responses
	if res.StatusCode >= 400 {
		message := string(body)
		_error.Message = message
		_error.Status = res.Status
		_error.Err = err
		_error.Request = req
		_error.Response = res
		slog.Error(_error.Error())
		slog.Debug(_error.Debug())
		return nil, &_error
	}

	err = json.Unmarshal(body, responseModel)
	if err != nil {
		_error.Message = err.Error()
		_error.Status = res.Status
		_error.Err = err
		_error.Request = req
		_error.Response = res
		slog.Error(_error.Error())
		slog.Debug(_error.Debug())
		return nil, &_error
	}

	return responseModel, nil
}

func (c AzureClient) GetEligibleResourceAssignments(token string) (*ResourceAssignmentResponse, error) {
	params := map[string]string{
		"api-version": AZ_PIM_API_VERSION,
		"$filter":     "asTarget()",
	}
	resp, err := Request(&PIMRequest{
		Url:    fmt.Sprintf("%s/%s/roleEligibilityScheduleInstances", c.ARMBaseURL, ARM_BASE_PATH),
		Token:  token,
		Method: "GET",
		Params: params,
	}, &ResourceAssignmentResponse{})
	if err != nil {
		return nil, err
	}
	return resp.(*ResourceAssignmentResponse), nil
}

func GetEligibleResourceAssignments(token string, c Client) (*ResourceAssignmentResponse, error) {
	return c.GetEligibleResourceAssignments(token)
}

func (c AzureClient) GetEligibleGovernanceRoleAssignments(roleType string, subjectId string, token string) (*GovernanceRoleAssignmentResponse, error) {
	if !IsGovernanceRoleType(roleType) {
		return nil, &Error{
			Operation: "GetEligibleGovernanceRoleAssignments",
			Message:   "Invalid role type specified.",
		}
	}
	params := map[string]string{
		"$expand": "linkedEligibleRoleAssignment,subject,scopedResource,roleDefinition($expand=resource)",
		"$filter": fmt.Sprintf("(subject/id eq '%s') and (assignmentState eq 'Eligible')", subjectId),
	}
	resp, err := Request(&PIMRequest{
		Url:    fmt.Sprintf("%s/%s/%s/roleAssignments", AZ_RBAC_BASE_URL, AZ_RBAC_BASE_PATH, roleType),
		Token:  token,
		Method: "GET",
		Params: params,
	}, &GovernanceRoleAssignmentResponse{})
	if err != nil {
		return nil, err
	}
	return resp.(*GovernanceRoleAssignmentResponse), nil
}

func GetEligibleGovernanceRoleAssignments(roleType string, subjectId string, token string, c Client) (*GovernanceRoleAssignmentResponse, error) {
	return c.GetEligibleGovernanceRoleAssignments(roleType, subjectId, token)
}

func (c AzureClient) ValidateResourceAssignmentRequest(scope string, resourceAssignmentRequest *ResourceAssignmentRequestRequest, token string) (bool, error) {
	params := map[string]string{
		"api-version": AZ_PIM_API_VERSION,
	}

	resourceAssignmentValidationRequest := resourceAssignmentRequest
	resourceAssignmentValidationRequest.Properties.IsValidationOnly = true

	resp, err := Request(&PIMRequest{
		Url: fmt.Sprintf(
			"%s/%s/%s/roleAssignmentScheduleRequests/%s/validate",
			c.ARMBaseURL,
			scope,
			ARM_BASE_PATH,
			uuid.NewString(),
		),
		Token:   token,
		Method:  "POST",
		Params:  params,
		Payload: resourceAssignmentValidationRequest,
	}, &ResourceAssignmentRequestResponse{})
	if err != nil {
		return false, err
	}
	validationResponse := resp.(*ResourceAssignmentRequestResponse)
	return validationResponse.CheckResourceAssignmentResult(resourceAssignmentValidationRequest), nil
}

func ValidateResourceAssignmentRequest(scope string, resourceAssignmentRequest *ResourceAssignmentRequestRequest, token string, c Client) (bool, error) {
	return c.ValidateResourceAssignmentRequest(scope, resourceAssignmentRequest, token)
}

func (c AzureClient) ValidateGovernanceRoleAssignmentRequest(roleType string, roleAssignmentRequest *GovernanceRoleAssignmentRequest, token string) (bool, error) {
	params := map[string]string{
		"evaluateOnly": "true",
	}

	governanceRoleAssignmentValidationRequest := roleAssignmentRequest

	resp, err := Request(&PIMRequest{
		Url:     fmt.Sprintf("%s/%s/%s/roleAssignmentRequests", AZ_RBAC_BASE_URL, AZ_RBAC_BASE_PATH, roleType),
		Token:   token,
		Method:  "POST",
		Params:  params,
		Payload: governanceRoleAssignmentValidationRequest,
	}, &GovernanceRoleAssignmentRequestResponse{})
	if err != nil {
		return false, err
	}
	validationResponse := resp.(*GovernanceRoleAssignmentRequestResponse)
	return validationResponse.CheckGovernanceRoleAssignmentResult(governanceRoleAssignmentValidationRequest), nil
}

func ValidateGovernanceRoleAssignmentRequest(roleType string, roleAssignmentRequest *GovernanceRoleAssignmentRequest, token string, c Client) (bool, error) {
	return c.ValidateGovernanceRoleAssignmentRequest(roleType, roleAssignmentRequest, token)
}

func (c AzureClient) RequestResourceAssignment(scope string, resourceAssignmentRequest *ResourceAssignmentRequestRequest, token string) (*ResourceAssignmentRequestResponse, error) {
	params := map[string]string{
		"api-version": AZ_PIM_API_VERSION,
	}

	resp, err := Request(&PIMRequest{
		Url: fmt.Sprintf(
			"%s/%s/%s/roleAssignmentScheduleRequests/%s",
			c.ARMBaseURL,
			scope,
			ARM_BASE_PATH,
			uuid.NewString(),
		),
		Token:   token,
		Method:  "PUT",
		Params:  params,
		Payload: resourceAssignmentRequest,
	}, &ResourceAssignmentRequestResponse{})
	if err != nil {
		return nil, err
	}
	responseModel := resp.(*ResourceAssignmentRequestResponse)
	responseModel.CheckResourceAssignmentResult(resourceAssignmentRequest)
	return responseModel, nil
}

func RequestResourceAssignment(scope string, resourceAssignmentRequest *ResourceAssignmentRequestRequest, token string, c Client) (*ResourceAssignmentRequestResponse, error) {
	return c.RequestResourceAssignment(scope, resourceAssignmentRequest, token)
}

func (c AzureClient) RequestGovernanceRoleAssignment(roleType string, governanceRoleAssignmentRequest *GovernanceRoleAssignmentRequest, token string) (*GovernanceRoleAssignmentRequestResponse, error) {
	resp, err := Request(&PIMRequest{
		Url:     fmt.Sprintf("%s/%s/%s/roleAssignmentRequests", AZ_RBAC_BASE_URL, AZ_RBAC_BASE_PATH, roleType),
		Token:   token,
		Method:  "POST",
		Payload: governanceRoleAssignmentRequest,
	}, &GovernanceRoleAssignmentRequestResponse{})
	if err != nil {
		return nil, err
	}
	responseModel := resp.(*GovernanceRoleAssignmentRequestResponse)
	responseModel.CheckGovernanceRoleAssignmentResult(governanceRoleAssignmentRequest)
	return responseModel, nil
}

func RequestGovernanceRoleAssignment(roleType string, governanceRoleAssignmentRequest *GovernanceRoleAssignmentRequest, token string, c Client) (*GovernanceRoleAssignmentRequestResponse, error) {
	return c.RequestGovernanceRoleAssignment(roleType, governanceRoleAssignmentRequest, token)
}
