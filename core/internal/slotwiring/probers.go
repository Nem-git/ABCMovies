package slotwiring

import (
	"fmt"

	"github.com/nem-git/abcmovies/core/internal/apiserver"
)

// probers maps an adapter name to the credential prober it validates linked
// accounts with. It is the same shape as the provider and catalogue registries
// above: an adapter registers its own prober from its own wiring file, so
// adding an adapter touches only that adapter's file — never a shared switch
// that every adapter's author must edit and merge around.
var probers = map[string]apiserver.CredentialProber{}

// RegisterProber records the credential prober for one adapter. Called from
// the adapter's wiring file via init(), alongside RegisterProvider. An adapter
// that does not validate a linked account's credentials by itself simply
// registers nothing (PLAN.md §3.5: the core never vaults material it has not
// confirmed works, so an adapter without a prober cannot be linked).
func RegisterProber(adapter string, p apiserver.CredentialProber) {
	if p == nil {
		panic(fmt.Sprintf("slotwiring: prober for adapter %q is nil", adapter))
	}
	if _, dup := probers[adapter]; dup {
		panic(fmt.Sprintf("slotwiring: prober for adapter %q registered twice", adapter))
	}
	probers[adapter] = p
}

// ProberForAdapter returns the credential prober registered for an adapter,
// or nil when the adapter does not validate a linked account's credentials by
// itself (PLAN.md §3.5: adapters without a prober cannot be linked). The key
// is the adapter name — the same value a LinkAccountRequest carries as its
// provider.
func ProberForAdapter(adapter string) apiserver.CredentialProber {
	return probers[adapter]
}
