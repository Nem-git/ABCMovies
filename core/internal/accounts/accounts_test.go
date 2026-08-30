package accounts

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/nem-git/abcmovies/core/internal/store"
)

func TestAddGetRoundtrip(t *testing.T) {
	st := NewStore(store.NewInMemory(), nil)
	ctx := context.Background()
	rec := Record{
		ID:          NewID(),
		Provider:    "jellyfin",
		BaseURL:     "http://jf.lan:8096",
		Username:    "bob",
		OwnerUserID: "user-1",
	}
	if err := st.Add(ctx, rec); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, err := st.Get(ctx, rec.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Provider != "jellyfin" || got.BaseURL != rec.BaseURL || got.Username != "bob" || got.OwnerUserID != "user-1" {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
	// Defaults applied: status linked, timestamp set.
	if got.Status != StatusLinked {
		t.Fatalf("status = %q, want linked", got.Status)
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("created_at was not stamped")
	}
}

func TestAddRejectsDuplicate(t *testing.T) {
	st := NewStore(store.NewInMemory(), nil)
	ctx := context.Background()
	rec := Record{ID: NewID(), Provider: "jellyfin", BaseURL: "http://jf", Username: "a"}
	if err := st.Add(ctx, rec); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	other := rec
	other.Username = "b"
	if err := st.Add(ctx, other); err == nil {
		t.Fatal("duplicate id Add succeeded")
	}
}

func TestAddValidatesRequiredFields(t *testing.T) {
	st := NewStore(store.NewInMemory(), nil)
	ctx := context.Background()
	base := Record{ID: NewID(), Provider: "jellyfin", BaseURL: "http://jf", Username: "a"}
	for _, mutate := range []func(*Record){
		func(r *Record) { r.ID = "" },
		func(r *Record) { r.Provider = "" },
		func(r *Record) { r.BaseURL = "" },
		func(r *Record) { r.Username = "" },
	} {
		rec := base
		mutate(&rec)
		if err := st.Add(ctx, rec); err == nil {
			t.Fatalf("Add with missing required field succeeded: %+v", rec)
		}
	}
}

func TestAddDefaultsVisibilityPrivate(t *testing.T) {
	st := NewStore(store.NewInMemory(), nil)
	ctx := context.Background()
	rec := Record{ID: NewID(), Provider: "jellyfin", BaseURL: "http://jf", Username: "a", OwnerUserID: "user-1"}
	if err := st.Add(ctx, rec); err != nil {
		t.Fatalf("Add: %v", err)
	}
	got, err := st.Get(ctx, rec.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Visibility != VisibilityPrivate {
		t.Fatalf("visibility = %q, want private (%q)", got.Visibility, VisibilityPrivate)
	}
}

func TestAddVisibilityRules(t *testing.T) {
	st := NewStore(store.NewInMemory(), nil)
	ctx := context.Background()

	t.Run("explicit values are retained", func(t *testing.T) {
		for _, vis := range []Visibility{VisibilityShared, VisibilityPublic} {
			rec := Record{ID: NewID(), Provider: "jellyfin", BaseURL: "http://jf", Username: "a", OwnerUserID: "user-1", Visibility: vis}
			if vis == VisibilityShared {
				rec.SharedWith = []string{"bob"}
			}
			if err := st.Add(ctx, rec); err != nil {
				t.Fatalf("Add (%q): %v", vis, err)
			}
			got, err := st.Get(ctx, rec.ID)
			if err != nil {
				t.Fatalf("Get (%q): %v", vis, err)
			}
			if got.Visibility != vis {
				t.Fatalf("visibility = %q, want %q", got.Visibility, vis)
			}
		}
	})

	t.Run("shared requires members", func(t *testing.T) {
		rec := Record{ID: NewID(), Provider: "jellyfin", BaseURL: "http://jf", Username: "a", Visibility: VisibilityShared}
		if err := st.Add(ctx, rec); err == nil {
			t.Fatal("Add with shared visibility but no members succeeded")
		}
	})

	t.Run("shared members are canonicalized", func(t *testing.T) {
		rec := Record{ID: NewID(), Provider: "jellyfin", BaseURL: "http://jf", Username: "a", Visibility: VisibilityShared, SharedWith: []string{"  bob ", "alice", "bob"}}
		if err := st.Add(ctx, rec); err != nil {
			t.Fatalf("Add: %v", err)
		}
		got, err := st.Get(ctx, rec.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if fmt.Sprint(got.SharedWith) != "[alice bob]" {
			t.Fatalf("shared_with = %v, want [alice bob]", got.SharedWith)
		}
	})

	t.Run("non-shared visibility clears members", func(t *testing.T) {
		rec := Record{ID: NewID(), Provider: "jellyfin", BaseURL: "http://jf", Username: "a", Visibility: VisibilityPrivate, SharedWith: []string{"bob"}}
		if err := st.Add(ctx, rec); err != nil {
			t.Fatalf("Add: %v", err)
		}
		got, err := st.Get(ctx, rec.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.Visibility != VisibilityPrivate || len(got.SharedWith) != 0 {
			t.Fatalf("private record kept members: %+v", got)
		}
	})

	t.Run("invalid visibility is rejected", func(t *testing.T) {
		rec := Record{ID: NewID(), Provider: "jellyfin", BaseURL: "http://jf", Username: "a", Visibility: "everyone"}
		if err := st.Add(ctx, rec); err == nil {
			t.Fatal("Add with invalid visibility succeeded")
		}
	})
}

func TestGetMissingIsNotFound(t *testing.T) {
	st := NewStore(store.NewInMemory(), nil)
	if _, err := st.Get(context.Background(), "lnk_never-minted"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get missing = %v, want ErrNotFound", err)
	}
}

func TestSetStatusAndListOrdering(t *testing.T) {
	st := NewStore(store.NewInMemory(), nil)
	ctx := context.Background()
	clock := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	st.now = func() time.Time { clock = clock.Add(time.Hour); return clock }

	first := Record{ID: NewID(), Provider: "jellyfin", BaseURL: "http://jf-a", Username: "a"}
	if err := st.Add(ctx, first); err != nil {
		t.Fatalf("Add first: %v", err)
	}
	second := Record{ID: NewID(), Provider: "jellyfin", BaseURL: "http://jf-b", Username: "b"}
	if err := st.Add(ctx, second); err != nil {
		t.Fatalf("Add second: %v", err)
	}

	if err := st.SetStatus(ctx, first.ID, StatusExpired); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	rec, err := st.Get(ctx, first.ID)
	if err != nil {
		t.Fatalf("Get after SetStatus: %v", err)
	}
	if rec.Status != StatusExpired {
		t.Fatalf("status = %q, want expired", rec.Status)
	}
	if rec.Username != "a" {
		t.Fatalf("SetStatus clobbered record; username = %q", rec.Username)
	}

	list, err := st.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 || list[0].ID != first.ID || list[1].ID != second.ID {
		t.Fatalf("List ordering wrong; oldest first expected: %+v", list)
	}
}

func TestDeleteRemovesRecordAndSession(t *testing.T) {
	st := NewStore(store.NewInMemory(), nil)
	ctx := context.Background()
	rec := Record{ID: NewID(), Provider: "jellyfin", BaseURL: "http://jf", Username: "a"}
	if err := st.Add(ctx, rec); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := st.Save(ctx, rec.ID, []byte("session-credential")); err != nil {
		t.Fatalf("Save session: %v", err)
	}

	if err := st.Delete(ctx, rec.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := st.Get(ctx, rec.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("record survives Delete: %v", err)
	}
	if _, err := st.Load(ctx, rec.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("session blob survives Delete: %v", err)
	}
}

func TestSessionBlobRoundtripUnderAccountID(t *testing.T) {
	vault := store.NewInMemory()
	st := NewStore(vault, nil)
	ctx := context.Background()
	id := NewID()
	if err := st.Save(ctx, id, []byte("vaulted-session")); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// The blob must sit exactly at the account id — the key a provider slot's
	// session vault reads back, so wiring finds it without a re-login.
	raw, err := vault.Get(ctx, id)
	if err != nil {
		t.Fatalf("blob not at account id key: %v", err)
	}
	if string(raw) != "vaulted-session" {
		t.Fatalf("blob content = %q", raw)
	}
	got, err := st.Load(ctx, id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if string(got) != "vaulted-session" {
		t.Fatalf("Load roundtrip = %q", got)
	}
}

func TestListSkipsUnreadableRecord(t *testing.T) {
	vault := store.NewInMemory()
	st := NewStore(vault, nil)
	ctx := context.Background()
	good := Record{ID: NewID(), Provider: "jellyfin", BaseURL: "http://jf-ok", Username: "a"}
	if err := st.Add(ctx, good); err != nil {
		t.Fatalf("Add good: %v", err)
	}
	// A torn record: right key, garbage body.
	if err := vault.Put(ctx, metaPrefix+"lnk_torn", []byte("{{{")); err != nil {
		t.Fatalf("seed torn record: %v", err)
	}

	list, err := st.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].ID != good.ID {
		t.Fatalf("List includes torn record: %+v", list)
	}
}

func TestNewIDPrefixesLinkedNamespace(t *testing.T) {
	id := NewID()
	if len(id) != len("lnk_")+32 {
		t.Fatalf("id shape wrong: %q", id)
	}
	if id[:4] != "lnk_" {
		t.Fatalf("id missing linked prefix: %q", id)
	}
	if another := NewID(); another == id {
		t.Fatal("NewID returned a duplicate")
	}
}
