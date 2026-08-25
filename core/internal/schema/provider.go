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
