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

// ValidateStartDeliveryRequest checks a StartDeliveryRequest (PLAN.md §6).
func ValidateStartDeliveryRequest(r *apiv1.StartDeliveryRequest) error {
	if r == nil {
		return fmt.Errorf("start_delivery_request: nil")
	}
	if r.GetGoal() == apiv1.DeliveryGoal_DELIVERY_GOAL_UNSPECIFIED {
		return fmt.Errorf("start_delivery_request: goal is required")
	}
	if r.GetProvider() == "" {
		return fmt.Errorf("start_delivery_request: provider is required")
	}
	if r.GetAccountId() == "" {
		return fmt.Errorf("start_delivery_request: account_id is required")
	}
	if r.GetMemberUserId() == "" {
		return fmt.Errorf("start_delivery_request: member_user_id is required")
	}
	if r.GetNativeId() == "" {
		return fmt.Errorf("start_delivery_request: native_id is required")
	}
	if r.GetSink() == "" {
		return fmt.Errorf("start_delivery_request: sink is required")
	}
	return nil
}

// ValidateStartDeliveryResponse checks a StartDeliveryResponse.
func ValidateStartDeliveryResponse(r *apiv1.StartDeliveryResponse) error {
	if r == nil {
		return fmt.Errorf("start_delivery_response: nil")
	}
	if r.GetJob() == nil {
		return fmt.Errorf("start_delivery_response: job is required")
	}
	return ValidateJob(r.GetJob())
}

// ValidateHeartbeatRequest checks a HeartbeatRequest (PLAN.md §9.1).
func ValidateHeartbeatRequest(r *apiv1.HeartbeatRequest) error {
	if r == nil {
		return fmt.Errorf("heartbeat_request: nil")
	}
	if r.GetSessionId() == "" {
		return fmt.Errorf("heartbeat_request: session_id is required")
	}
	return nil
}

// ValidateHeartbeatResponse checks a HeartbeatResponse.
func ValidateHeartbeatResponse(r *apiv1.HeartbeatResponse) error {
	if r == nil {
		return fmt.Errorf("heartbeat_response: nil")
	}
	if r.GetSessionId() == "" {
		return fmt.Errorf("heartbeat_response: session_id is required")
	}
	return nil
}
