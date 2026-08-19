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
