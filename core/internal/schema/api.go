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

// ValidateGetLibraryRequest checks a GetLibraryRequest (PLAN.md §5, §8).
func ValidateGetLibraryRequest(r *apiv1.GetLibraryRequest) error {
	if r == nil {
		return fmt.Errorf("get_library_request: nil")
	}
	return nil
}

// ValidateLinkAccountRequest checks a LinkAccountRequest (PLAN.md §3.5, §7.5).
func ValidateLinkAccountRequest(r *apiv1.LinkAccountRequest) error {
	if r == nil {
		return fmt.Errorf("link_account_request: nil")
	}
	if r.GetProvider() == "" {
		return fmt.Errorf("link_account_request: provider is required")
	}
	if r.GetBaseUrl() == "" {
		return fmt.Errorf("link_account_request: base_url is required")
	}
	pw := r.GetPassword()
	if pw == nil {
		return fmt.Errorf("link_account_request: password auth method is required")
	}
	if pw.GetUsername() == "" {
		return fmt.Errorf("link_account_request: username is required")
	}
	if len(pw.GetPassword()) == 0 {
		return fmt.Errorf("link_account_request: password is required")
	}
	if r.GetVisibility() == apiv1.AccountVisibility_ACCOUNT_VISIBILITY_SHARED && len(r.GetSharedWith()) == 0 {
		return fmt.Errorf("link_account_request: shared visibility requires shared_with users")
	}
	return nil
}

// ValidateListAccountsRequest checks a ListAccountsRequest (PLAN.md §7.5).
func ValidateListAccountsRequest(r *apiv1.ListAccountsRequest) error {
	if r == nil {
		return fmt.Errorf("list_accounts_request: nil")
	}
	return nil
}

// ValidateRemoveAccountRequest checks a RemoveAccountRequest (PLAN.md §7.5).
func ValidateRemoveAccountRequest(r *apiv1.RemoveAccountRequest) error {
	if r == nil {
		return fmt.Errorf("remove_account_request: nil")
	}
	if r.GetAccountId() == "" {
		return fmt.Errorf("remove_account_request: account_id is required")
	}
	return nil
}

// ValidateGetPlayInfoRequest checks a GetPlayInfoRequest (PLAN.md §6.1).
func ValidateGetPlayInfoRequest(r *apiv1.GetPlayInfoRequest) error {
	if r == nil {
		return fmt.Errorf("get_play_info_request: nil")
	}
	if r.GetSessionId() == "" {
		return fmt.Errorf("get_play_info_request: session_id is required")
	}
	return nil
}

// ValidateGetLibraryResponse checks a GetLibraryResponse (PLAN.md §5, §8).
func ValidateGetLibraryResponse(r *apiv1.GetLibraryResponse) error {
	if r == nil {
		return fmt.Errorf("get_library_response: nil")
	}
	for i, item := range r.GetItems() {
		if item == nil {
			return fmt.Errorf("get_library_response: item %d is nil", i)
		}
		if err := ValidateLibraryEntry(item.GetEntry()); err != nil {
			return err
		}
		if m := item.GetMetadata(); m != nil {
			if err := ValidateTitleMetadata(m); err != nil {
				return err
			}
		}
	}
	return nil
}

// ValidateLinkAccountResponse checks a LinkAccountResponse (PLAN.md §7.5).
func ValidateLinkAccountResponse(r *apiv1.LinkAccountResponse) error {
	if r == nil {
		return fmt.Errorf("link_account_response: nil")
	}
	if r.GetAccountId() == "" {
		return fmt.Errorf("link_account_response: account_id is required")
	}
	return nil
}

// ValidateListAccountsResponse checks a ListAccountsResponse (PLAN.md §7.5).
func ValidateListAccountsResponse(r *apiv1.ListAccountsResponse) error {
	if r == nil {
		return fmt.Errorf("list_accounts_response: nil")
	}
	for i, acct := range r.GetAccounts() {
		if acct == nil {
			return fmt.Errorf("list_accounts_response: account %d is nil", i)
		}
		if acct.GetAccountId() == "" {
			return fmt.Errorf("list_accounts_response: account %d: account_id is required", i)
		}
		if acct.GetProvider() == "" {
			return fmt.Errorf("list_accounts_response: account %d: provider is required", i)
		}
	}
	return nil
}

// ValidateGetPlayInfoResponse checks a GetPlayInfoResponse (PLAN.md §6.1).
func ValidateGetPlayInfoResponse(r *apiv1.GetPlayInfoResponse) error {
	if r == nil {
		return fmt.Errorf("play_info_response: nil")
	}
	for i, t := range r.GetTracks() {
		if t == nil {
			return fmt.Errorf("play_info_response: track %d is nil", i)
		}
		if t.GetTrackId() == "" {
			return fmt.Errorf("play_info_response: track %d: track_id is required", i)
		}
		if t.GetRelayUrl() == "" {
			return fmt.Errorf("play_info_response: track %d: relay_url is required", i)
		}
	}
	return nil
}
