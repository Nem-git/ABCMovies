// Package mediaserver implements the media-server sink contract
// (PLAN.md §6.4): a pull sink that stages one relay ticket per located track,
// so any conformant player can open them and do its own language, subtitle,
// and quality selection and ABR with no provider-specific knowledge.
//
// The package is pure — generated protos and stdlib only; the delivery
// package wires it to the relay and opener (adapters never import core
// internals). The sink stages separate tracks, so the engine packages them
// into a manifest (the Package pipeline step) when the player requests one.
package mediaserver
