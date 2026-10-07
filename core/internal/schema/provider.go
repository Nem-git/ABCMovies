package schema

import (
	"fmt"

	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// ValidateCatalogueSyncRequest checks a CatalogueSyncRequest. The account is
// required: an unknown account is a runtime error, never an implicit default
// (PLAN.md §5.4).
func ValidateCatalogueSyncRequest(req *slotsv1.CatalogueSyncRequest) error {
	if req == nil {
		return fmt.Errorf("catalogue sync request: nil")
	}
	if req.GetAccountId() == "" {
		return fmt.Errorf("catalogue sync request: account_id is required")
	}
	return nil
}

// ValidateCatalogueItem checks one provider-native catalogue item. The
// identity-relevant minimum is enforced here; full TitleMetadata validation
// applies at cache ingest, not on the sync wire.
func ValidateCatalogueItem(item *slotsv1.CatalogueItem) error {
	if item == nil {
		return fmt.Errorf("catalogue item: nil")
	}
	if item.GetNativeId() == "" {
		return fmt.Errorf("catalogue item: native_id is required")
	}
	if item.GetKind() == slotsv1.ItemKind_ITEM_KIND_UNSPECIFIED {
		return fmt.Errorf("catalogue item %q: kind is required", item.GetNativeId())
	}
	if item.GetMetadata() == nil {
		return fmt.Errorf("catalogue item %q: metadata is required", item.GetNativeId())
	}
	if item.GetMetadata().GetTitle() == "" {
		return fmt.Errorf("catalogue item %q: metadata.title is required", item.GetNativeId())
	}
	for i, id := range item.GetExternalIds() {
		if id == nil {
			return fmt.Errorf("catalogue item %q: external id %d is nil", item.GetNativeId(), i)
		}
		if id.GetNamespace() == "" || id.GetValue() == "" {
			return fmt.Errorf("catalogue item %q: external id %d requires namespace and value", item.GetNativeId(), i)
		}
	}
	return nil
}

// ValidateCatalogueSyncResponse checks one page of a catalogue sync. A page
// that lists the same native item twice is malformed and must be rejected,
// never downgraded (PLAN.md §2.5).
func ValidateCatalogueSyncResponse(resp *slotsv1.CatalogueSyncResponse) error {
	if resp == nil {
		return fmt.Errorf("catalogue sync response: nil")
	}
	seen := make(map[string]struct{}, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		if err := ValidateCatalogueItem(item); err != nil {
			return err
		}
		if _, dup := seen[item.GetNativeId()]; dup {
			return fmt.Errorf("catalogue sync response: duplicate native_id %q on one page", item.GetNativeId())
		}
		seen[item.GetNativeId()] = struct{}{}
	}
	return nil
}

// ValidateProduceSourcesRequest checks a ProduceSourcesRequest. Both the
// account whose session resolves the source and the item's provider-native id
// are required (PLAN.md §6.2).
func ValidateProduceSourcesRequest(req *slotsv1.ProduceSourcesRequest) error {
	if req == nil {
		return fmt.Errorf("produce sources request: nil")
	}
	if req.GetAccountId() == "" {
		return fmt.Errorf("produce sources request: account_id is required")
	}
	if req.GetNativeId() == "" {
		return fmt.Errorf("produce sources request: native_id is required")
	}
	return nil
}

// ValidateProduceSourcesResponse checks a manifest. The manifest itself is
// validated against the MediaSource contract (PLAN.md §6.2); a missing
// manifest is rejected, never downgraded.
func ValidateProduceSourcesResponse(resp *slotsv1.ProduceSourcesResponse) error {
	if resp == nil {
		return fmt.Errorf("produce sources response: nil")
	}
	if resp.GetSource() == nil {
		return fmt.Errorf("produce sources response: source is required")
	}
	return ValidateMediaSource(resp.GetSource())
}

// ValidateSearchCatalogRequest checks a SearchCatalogRequest: the account
// and the user's query are required (PLAN.md §5.4 — the live path is
// always user-triggered, never a background probe).
func ValidateSearchCatalogRequest(req *slotsv1.SearchCatalogRequest) error {
	if req == nil {
		return fmt.Errorf("search catalog request: nil")
	}
	if req.GetAccountId() == "" {
		return fmt.Errorf("search catalog request: account_id is required")
	}
	if req.GetQuery() == "" {
		return fmt.Errorf("search catalog request: query is required")
	}
	return nil
}

// ValidateSearchCatalogResponse checks one page of search matches.
func ValidateSearchCatalogResponse(resp *slotsv1.SearchCatalogResponse) error {
	if resp == nil {
		return fmt.Errorf("search catalog response: nil")
	}
	for _, item := range resp.GetItems() {
		if err := ValidateCatalogueItem(item); err != nil {
			return err
		}
	}
	return nil
}

// ValidateBrowseCatalogRequest checks a BrowseCatalogRequest: the account is
// required; the path may be empty (provider default landing).
func ValidateBrowseCatalogRequest(req *slotsv1.BrowseCatalogRequest) error {
	if req == nil {
		return fmt.Errorf("browse catalog request: nil")
	}
	if req.GetAccountId() == "" {
		return fmt.Errorf("browse catalog request: account_id is required")
	}
	return nil
}

// ValidateBrowseCatalogResponse checks one page of a browse listing.
func ValidateBrowseCatalogResponse(resp *slotsv1.BrowseCatalogResponse) error {
	if resp == nil {
		return fmt.Errorf("browse catalog response: nil")
	}
	for _, item := range resp.GetItems() {
		if err := ValidateCatalogueItem(item); err != nil {
			return err
		}
	}
	return nil
}

// ValidateRefreshAvailabilityRequest checks a RefreshAvailabilityRequest:
// both the account and at least one native id are required (PLAN.md §5.4 —
// the lookup is always bounded and explicit).
func ValidateRefreshAvailabilityRequest(req *slotsv1.RefreshAvailabilityRequest) error {
	if req == nil {
		return fmt.Errorf("refresh availability request: nil")
	}
	if req.GetAccountId() == "" {
		return fmt.Errorf("refresh availability request: account_id is required")
	}
	if len(req.GetNativeIds()) == 0 {
		return fmt.Errorf("refresh availability request: native_ids must be non-empty")
	}
	return nil
}

// ValidateRefreshAvailabilityResponse checks the availability answer: the
// carried items are the items still present on the provider.
func ValidateRefreshAvailabilityResponse(resp *slotsv1.RefreshAvailabilityResponse) error {
	if resp == nil {
		return fmt.Errorf("refresh availability response: nil")
	}
	for _, item := range resp.GetItems() {
		if err := ValidateCatalogueItem(item); err != nil {
			return err
		}
	}
	return nil
}
