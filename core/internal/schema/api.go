package schema

import (
	"fmt"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
)

// ValidateGetJobRequest checks a GetJobRequest.
func ValidateGetJobRequest(r *apiv1.GetJobRequest) error {
	if r == nil {
		return fmt.Errorf("get_job_request: nil")
	}
	if r.GetJobId() == "" {
		return fmt.Errorf("get_job_request: job_id is required")
	}
	return nil
}

// ValidateGetJobResponse checks a GetJobResponse.
func ValidateGetJobResponse(r *apiv1.GetJobResponse) error {
	if r == nil {
		return fmt.Errorf("get_job_response: nil")
	}
	if j := r.GetJob(); j != nil {
		return ValidateJob(j)
	}
	return nil
}

// ValidateSubscribeRequest checks a SubscribeRequest.
func ValidateSubscribeRequest(r *apiv1.SubscribeRequest) error {
	if r == nil {
		return fmt.Errorf("subscribe_request: nil")
	}
	return nil
}

// ValidateSignUpRequest checks a SignUpRequest.
func ValidateSignUpRequest(r *apiv1.SignUpRequest) error {
	if r == nil {
		return fmt.Errorf("sign_up_request: nil")
	}
	if r.GetUsername() == "" {
		return fmt.Errorf("sign_up_request: username is required")
	}
	if r.GetPassword() == nil {
		return fmt.Errorf("sign_up_request: password method is required")
	}
	if len(r.GetPassword().GetPassword()) == 0 {
		return fmt.Errorf("sign_up_request: password is required")
	}
	return nil
}

// ValidateLoginRequest checks a LoginRequest.
func ValidateLoginRequest(r *apiv1.LoginRequest) error {
	if r == nil {
		return fmt.Errorf("login_request: nil")
	}
	if r.GetUsername() == "" {
		return fmt.Errorf("login_request: username is required")
	}
	if r.GetPassword() == nil {
		return fmt.Errorf("login_request: password method is required")
	}
	if len(r.GetPassword().GetPassword()) == 0 {
		return fmt.Errorf("login_request: password is required")
	}
	return nil
}
