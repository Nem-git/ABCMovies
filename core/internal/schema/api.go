package schema

import (
	"fmt"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"github.com/nem-git/abcmovies/core/internal/slotvalidation"
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
	if r.GetNativeId() == "" {
		return fmt.Errorf("start_delivery_request: native_id is required")
	}
	if r.GetSink() == "" {
		return fmt.Errorf("start_delivery_request: sink is required")
	}
	return nil
}

// ValidateCancelDeliveryRequest checks a CancelDeliveryRequest.
func ValidateCancelDeliveryRequest(r *apiv1.CancelDeliveryRequest) error {
	if r == nil {
		return fmt.Errorf("cancel_delivery_request: nil")
	}
	if r.GetJobId() == "" {
		return fmt.Errorf("cancel_delivery_request: job_id is required")
	}
	return nil
}

// --- Slot admin validation (D8) ---

func ValidateCreateSlotRequest(r *apiv1.CreateSlotRequest) error {
	if r == nil {
		return fmt.Errorf("create_slot_request: nil")
	}
	if r.GetSlot() == nil {
		return fmt.Errorf("create_slot_request: slot is required")
	}
	return nil
}

func ValidateGetSlotRequest(r *apiv1.GetSlotRequest) error {
	if r == nil {
		return fmt.Errorf("get_slot_request: nil")
	}
	if r.GetRef() == nil {
		return fmt.Errorf("get_slot_request: ref is required")
	}
	if r.GetRef().GetId() == "" {
		return fmt.Errorf("get_slot_request: id is required")
	}
	return nil
}

func ValidateUpdateSlotRequest(r *apiv1.UpdateSlotRequest) error {
	if r == nil {
		return fmt.Errorf("update_slot_request: nil")
	}
	if r.GetSlot() == nil {
		return fmt.Errorf("update_slot_request: slot is required")
	}
	return nil
}

func ValidateDeleteSlotRequest(r *apiv1.DeleteSlotRequest) error {
	if r == nil {
		return fmt.Errorf("delete_slot_request: nil")
	}
	if r.GetRef() == nil {
		return fmt.Errorf("delete_slot_request: ref is required")
	}
	if r.GetRef().GetId() == "" {
		return fmt.Errorf("delete_slot_request: id is required")
	}
	return nil
}

func ValidateListSlotsRequest(r *apiv1.ListSlotsRequest) error {
	if r == nil {
		return fmt.Errorf("list_slots_request: nil")
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

// ValidateAccountSharing checks a wholesale sharing replacement: the
// visibility must be one of the three real values, and the member list must
// be present exactly when the visibility is shared — an orphan roster is an
// ambiguous request, not a silently empty one.
func ValidateAccountSharing(s *apiv1.AccountSharing) error {
	if s == nil {
		return fmt.Errorf("account_sharing: nil")
	}
	switch s.GetVisibility() {
	case apiv1.AccountVisibility_ACCOUNT_VISIBILITY_PRIVATE,
		apiv1.AccountVisibility_ACCOUNT_VISIBILITY_PUBLIC:
		if len(s.GetSharedWith()) != 0 {
			return fmt.Errorf("account_sharing: shared_with is only meaningful with shared visibility")
		}
	case apiv1.AccountVisibility_ACCOUNT_VISIBILITY_SHARED:
		if len(s.GetSharedWith()) == 0 {
			return fmt.Errorf("account_sharing: shared visibility requires shared_with users")
		}
	default:
		return fmt.Errorf("account_sharing: visibility is required")
	}
	return nil
}

// ValidateUpdateAccountRequest checks an UpdateAccountRequest (PLAN.md §7.1,
// §7.2): the account id is required, every present field is well-formed, the
// policy value is one of the known choices, and at least one setting must
// actually be changing — a call that changes nothing is a caller bug, and
// the handler would otherwise be a no-op dressed as success.
func ValidateUpdateAccountRequest(r *apiv1.UpdateAccountRequest) error {
	if r == nil {
		return fmt.Errorf("update_account_request: nil")
	}
	if r.GetAccountId() == "" {
		return fmt.Errorf("update_account_request: account_id is required")
	}
	if r.GetSharing() != nil {
		if err := ValidateAccountSharing(r.GetSharing()); err != nil {
			return fmt.Errorf("update_account_request: %w", err)
		}
	}
	if r.MaxConcurrentStreams == nil && r.GetSharing() == nil && r.CapChangePolicy == nil {
		return fmt.Errorf("update_account_request: at least one setting must change")
	}
	if r.CapChangePolicy != nil {
		switch *r.CapChangePolicy {
		case apiv1.CapChangePolicy_CAP_CHANGE_POLICY_UNSPECIFIED,
			apiv1.CapChangePolicy_CAP_CHANGE_POLICY_NEW_SESSIONS_ONLY,
			apiv1.CapChangePolicy_CAP_CHANGE_POLICY_ENFORCE_NOW:
		default:
			return fmt.Errorf("update_account_request: unknown cap_change_policy %d", *r.CapChangePolicy)
		}
	}
	return nil
}

// ValidateUpdateAccountResponse checks an UpdateAccountResponse.
func ValidateUpdateAccountResponse(r *apiv1.UpdateAccountResponse) error {
	if r == nil {
		return fmt.Errorf("update_account_response: nil")
	}
	if r.GetAccount() == nil || r.GetAccount().GetAccountId() == "" {
		return fmt.Errorf("update_account_response: account is required")
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
// The menu is a oneof: exactly one branch is set — a raw per-track list, or a
// packaged manifest URL. Each branch is validated for its own required fields.
func ValidateGetPlayInfoResponse(r *apiv1.GetPlayInfoResponse) error {
	if r == nil {
		return fmt.Errorf("play_info_response: nil")
	}
	switch m := r.GetMenu().(type) {
	case *apiv1.GetPlayInfoResponse_RawTracks:
		for i, t := range m.RawTracks.GetTracks() {
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
	case *apiv1.GetPlayInfoResponse_ManifestUrl:
		if m.ManifestUrl == "" {
			return fmt.Errorf("play_info_response: manifest_url is required")
		}
	default:
		return fmt.Errorf("play_info_response: menu must carry one branch (tracks or manifest_url)")
	}
	return nil
}

// --- Slot admin responses (D8) ---

// ValidateCreateSlotResponse checks a CreateSlotResponse: the created slot
// must itself be valid, so a response can never carry a slot the store would
// reject.
func ValidateCreateSlotResponse(r *apiv1.CreateSlotResponse) error {
	if r == nil {
		return fmt.Errorf("create_slot_response: nil")
	}
	if r.GetSlot() == nil {
		return fmt.Errorf("create_slot_response: slot is required")
	}
	return slotvalidation.ValidateSlot(r.GetSlot())
}

// ValidateGetSlotResponse checks a GetSlotResponse.
func ValidateGetSlotResponse(r *apiv1.GetSlotResponse) error {
	if r == nil {
		return fmt.Errorf("get_slot_response: nil")
	}
	if r.GetSlot() == nil {
		return fmt.Errorf("get_slot_response: slot is required")
	}
	return slotvalidation.ValidateSlot(r.GetSlot())
}

// ValidateUpdateSlotResponse checks an UpdateSlotResponse.
func ValidateUpdateSlotResponse(r *apiv1.UpdateSlotResponse) error {
	if r == nil {
		return fmt.Errorf("update_slot_response: nil")
	}
	if r.GetSlot() == nil {
		return fmt.Errorf("update_slot_response: slot is required")
	}
	return slotvalidation.ValidateSlot(r.GetSlot())
}

// ValidateDeleteSlotResponse checks a DeleteSlotResponse.
func ValidateDeleteSlotResponse(r *apiv1.DeleteSlotResponse) error {
	if r == nil {
		return fmt.Errorf("delete_slot_response: nil")
	}
	if r.GetSlot() == nil {
		return fmt.Errorf("delete_slot_response: slot is required")
	}
	return slotvalidation.ValidateSlot(r.GetSlot())
}

// ValidateListSlotsResponse checks a ListSlotsResponse: every listed slot
// must be valid.
func ValidateListSlotsResponse(r *apiv1.ListSlotsResponse) error {
	if r == nil {
		return fmt.Errorf("list_slots_response: nil")
	}
	for i, s := range r.GetSlots() {
		if s == nil {
			return fmt.Errorf("list_slots_response: slot %d is nil", i)
		}
		if err := slotvalidation.ValidateSlot(s); err != nil {
			return fmt.Errorf("list_slots_response: slot %d: %w", i, err)
		}
	}
	return nil
}
