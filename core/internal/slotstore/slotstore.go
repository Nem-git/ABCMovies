// Package slotstore holds the durable slot records (PLAN.md §3.1, D8). A
// slot is one managed instance of a slot kind, stored as a versioned JSON
// envelope so the schema can evolve without a migration.
//
// The slot id is the identity namespace (PLAN.md §1.25): items merge under
// it and metadata provenance is stamped by it. It is therefore globally
// unique across every kind and owner — two slots may never share an id, or
// their identities would collide. The store keys by id alone and enforces
// that uniqueness on write.
package slotstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/slotvalidation"
	"github.com/nem-git/abcmovies/core/internal/store"
)

// Store holds the durable slot records over a backend store.
type Store struct {
	backend store.Store
}

// New builds a Store over backend. Absence is normal.
func New(backend store.Store) (*Store, error) {
	if backend == nil {
		return nil, errors.New("slotstore: store is required")
	}
	return &Store{backend: backend}, nil
}

// key derives the store key for a slot. The id is globally unique, so it is
// the whole key — kind and owner are attributes of the record, not parts of
// its identity.
func key(id string) string {
	return fmt.Sprintf("slot:%s", id)
}

// envelope is the versioned storage wrapper. The version field lets the
// schema evolve without a migration: the store reads the version and
// dispatches to the right decoder.
type envelope struct {
	Version int32           `json:"version"`
	Slot    json.RawMessage `json:"slot"`
}

// Put stores a slot, overwriting the record with the same id only when it is
// the same slot (same kind and owner — an update). A different slot already
// holding the id is refused: the id is the identity namespace, so a second
// slot may never claim it. The slot is validated before it is stored.
func (s *Store) Put(ctx context.Context, slot *corev1.Slot) error {
	if slot == nil {
		return errors.New("slotstore: slot is required")
	}
	if err := slotvalidation.ValidateSlot(slot); err != nil {
		return err
	}
	k := key(slot.GetId())
	if raw, err := s.backend.Get(ctx, k); err == nil {
		existing, err := decodeSlot(raw)
		if err != nil {
			return fmt.Errorf("slotstore: decode existing slot %q: %w", slot.GetId(), err)
		}
		if existing.GetKind() != slot.GetKind() || existing.GetOwnerUserId() != slot.GetOwnerUserId() {
			return fmt.Errorf("slotstore: slot id %q already exists", slot.GetId())
		}
	} else if !errors.Is(err, store.ErrKeyNotFound) {
		return fmt.Errorf("slotstore: read existing slot %q: %w", slot.GetId(), err)
	}
	slotJSON, err := protojson.Marshal(slot)
	if err != nil {
		return fmt.Errorf("slotstore: marshal slot: %w", err)
	}
	raw, err := json.Marshal(envelope{Version: 1, Slot: slotJSON})
	if err != nil {
		return fmt.Errorf("slotstore: marshal envelope: %w", err)
	}
	return s.backend.Put(ctx, k, raw)
}

// GetByID reads the slot holding id, regardless of kind or owner. It is the
// duplicate check for a create: because the id is globally unique, any hit
// means the id is taken. Returns (nil, nil) if no slot holds the id.
func (s *Store) GetByID(ctx context.Context, id string) (*corev1.Slot, error) {
	raw, err := s.backend.Get(ctx, key(id))
	if errors.Is(err, store.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return decodeSlot(raw)
}

// Get reads a slot by kind, owner, and id. The id alone locates the record
// (ids are globally unique); kind and owner must then match, so a request
// aimed at a slot the id does not own answers as absent. Returns (nil, nil)
// if no matching slot exists.
func (s *Store) Get(ctx context.Context, kind corev1.SlotKind, owner, id string) (*corev1.Slot, error) {
	slot, err := s.GetByID(ctx, id)
	if err != nil || slot == nil {
		return nil, err
	}
	if slot.GetKind() != kind || slot.GetOwnerUserId() != owner {
		return nil, nil
	}
	return slot, nil
}

// Delete removes a slot. The id locates the record; it is deleted only when
// kind and owner also match, so a caller can never remove a slot the id does
// not belong to. Deleting a non-existent slot is a no-op.
func (s *Store) Delete(ctx context.Context, kind corev1.SlotKind, owner, id string) error {
	slot, err := s.Get(ctx, kind, owner, id)
	if err != nil || slot == nil {
		return err
	}
	return s.backend.Delete(ctx, key(id))
}

// ListByKind returns every slot of a kind, optionally filtered by owner. An
// empty owner lists every owner. Ids are globally unique, so this scans the
// slot keyspace and filters in memory; the set of slots is small.
func (s *Store) ListByKind(ctx context.Context, kind corev1.SlotKind, owner string) ([]*corev1.Slot, error) {
	keys, err := s.backend.List(ctx, "slot:")
	if err != nil {
		return nil, err
	}
	var slots []*corev1.Slot
	for _, k := range keys {
		raw, err := s.backend.Get(ctx, k)
		if err != nil {
			continue
		}
		slot, err := decodeSlot(raw)
		if err != nil {
			continue
		}
		if slot.GetKind() != kind {
			continue
		}
		if owner != "" && slot.GetOwnerUserId() != owner {
			continue
		}
		slots = append(slots, slot)
	}
	return slots, nil
}

// decodeSlot reads a versioned envelope and returns the slot.
func decodeSlot(raw []byte) (*corev1.Slot, error) {
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("slotstore: decode envelope: %w", err)
	}
	var slot corev1.Slot
	if err := protojson.Unmarshal(env.Slot, &slot); err != nil {
		return nil, fmt.Errorf("slotstore: decode slot: %w", err)
	}
	return &slot, nil
}
