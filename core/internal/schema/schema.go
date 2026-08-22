// Package schema validates the load-bearing contracts (docs/PLAN.md §2.3)
// against the rules the docs fix. A message that passes validation is a valid
// instance of the exact version of the contract it declares; a message that
// fails must be rejected, never downgraded (PLAN.md §2.5).
//
// The rules here are the mechanical reading of the reference documents; the
// fixture suites (fixtures/<contract>/v1/) are the approval gate that pins
// them. Each validation rule maps to at least one fixture.
package schema

import (
	"fmt"
	"strings"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

// ValidateLibraryEntry checks a LibraryEntry (PLAN.md §2.3, §5.3).
func ValidateLibraryEntry(le *corev1.LibraryEntry) error {
	if le == nil {
		return fmt.Errorf("library entry: nil")
	}
	if le.GetId() == "" {
		return fmt.Errorf("library entry: id is required")
	}
	if !strings.HasPrefix(le.GetId(), "le_") {
		return fmt.Errorf("library entry: id must start with \"le_\" (PLAN.md §2.3)")
	}
	if le.GetKind() == corev1.EntryKind_ENTRY_KIND_UNSPECIFIED {
		return fmt.Errorf("library entry: kind is required")
	}
	for key, row := range le.GetCoverage() {
		if err := validateCoverageRow(le.GetKind(), key, row); err != nil {
			return err
		}
	}
	for i, id := range le.GetExternalIdentities() {
		if err := validateExternalIdentity(i, id); err != nil {
			return err
		}
	}
	return nil
}

func validateCoverageRow(kind corev1.EntryKind, key string, row *corev1.CoverageRow) error {
	if row == nil {
		return fmt.Errorf("library entry: coverage row %q is nil", key)
	}
	if row.GetVerdict() == corev1.CoverageVerdict_COVERAGE_VERDICT_UNSPECIFIED {
		return fmt.Errorf("library entry: coverage row %q: verdict is required", key)
	}
	if row.GetPresent() && row.GetVia() == "" {
		return fmt.Errorf("library entry: coverage row %q: a present row requires provenance (via)", key)
	}
	if len(row.GetSeasons()) > 0 && kind == corev1.EntryKind_ENTRY_KIND_MOVIE {
		return fmt.Errorf("library entry: coverage row %q: seasons are series-only", key)
	}
	for i, s := range row.GetSeasons() {
		if s == nil {
			return fmt.Errorf("library entry: coverage row %q: season range %d is nil", key, i)
		}
		if s.GetStart() > s.GetEnd() {
			return fmt.Errorf("library entry: coverage row %q: season range %d: start %d > end %d", key, i, s.GetStart(), s.GetEnd())
		}
	}
	return nil
}

func validateExternalIdentity(i int, id *corev1.ExternalIdentity) error {
	if id == nil {
		return fmt.Errorf("library entry: external identity %d is nil", i)
	}
	if id.GetNamespace() == "" || id.GetValue() == "" || id.GetProvenance() == "" {
		return fmt.Errorf("library entry: external identity %d: namespace, value, and provenance are required", i)
	}
	if id.GetVerdict() == corev1.IdentityVerdict_IDENTITY_VERDICT_UNSPECIFIED {
		return fmt.Errorf("library entry: external identity %d: verdict is required", i)
	}
	return nil
}

// ValidateMediaSource checks a MediaSource manifest (PLAN.md §6.2).
func ValidateMediaSource(ms *corev1.MediaSource) error {
	if ms == nil {
		return fmt.Errorf("media source: nil")
	}
	if ms.GetType() == corev1.MediaSourceType_MEDIA_SOURCE_TYPE_UNSPECIFIED {
		return fmt.Errorf("media source: type is required")
	}
	if ms.GetSeekable() == corev1.Seekable_SEEKABLE_UNSPECIFIED {
		return fmt.Errorf("media source: seekable is required")
	}
	if ms.GetAddressable() == corev1.Addressable_ADDRESSABLE_UNSPECIFIED {
		return fmt.Errorf("media source: addressable is required")
	}
	if ms.GetSeekable() == corev1.Seekable_SEEKABLE_DVR_WINDOW && ms.GetDvrWindow() == nil {
		return fmt.Errorf("media source: dvr_window is required when seekable is DVR_WINDOW")
	}
	tracks := ms.GetTracks()
	if len(tracks) == 0 {
		return fmt.Errorf("media source: at least one track is required")
	}

	ids := make(map[string]struct{}, len(tracks))
	for _, t := range tracks {
		if err := validateTrackID(t, ids); err != nil {
			return err
		}
	}

	hasLocationCarrier := false
	for _, t := range tracks {
		sub := t.GetSubtitle()
		delivery := t.GetDelivery()
		if sub == nil && delivery == nil {
			return fmt.Errorf("media source: track %q: a video/audio track requires delivery (only burned-in subtitle tracks carry none)", t.GetId())
		}
		if sub != nil {
			if err := validateSubtitleTrack(t.GetId(), sub, delivery); err != nil {
				return err
			}
		}
		if delivery != nil {
			if len(delivery.GetLocations()) > 0 {
				hasLocationCarrier = true
			}
			if err := validateDelivery(ms.GetAddressable(), t.GetId(), delivery, ids); err != nil {
				return err
			}
		}
	}
	if ms.GetAddressable() == corev1.Addressable_ADDRESSABLE_WHOLE_MUX && !hasLocationCarrier {
		return fmt.Errorf("media source: a WHOLE_MUX manifest requires at least one track with locations")
	}
	return nil
}

func validateTrackID(t *corev1.Track, ids map[string]struct{}) error {
	if t == nil {
		return fmt.Errorf("media source: nil track")
	}
	if t.GetId() == "" {
		return fmt.Errorf("media source: track id is required")
	}
	if _, dup := ids[t.GetId()]; dup {
		return fmt.Errorf("media source: duplicate track id %q", t.GetId())
	}
	if t.GetVideo() == nil && t.GetAudio() == nil && t.GetSubtitle() == nil {
		return fmt.Errorf("media source: track %q: exactly one media type is required", t.GetId())
	}
	ids[t.GetId()] = struct{}{}
	return nil
}

func validateSubtitleTrack(id string, sub *corev1.SubtitleTrack, delivery *corev1.TrackDelivery) error {
	role := sub.GetRole()
	if role == corev1.SubtitleRole_SUBTITLE_ROLE_UNSPECIFIED {
		role = corev1.SubtitleRole_SUBTITLE_ROLE_SUBTITLE
	}
	if sub.GetForced() && role != corev1.SubtitleRole_SUBTITLE_ROLE_SUBTITLE {
		return fmt.Errorf("media source: track %q: forced subtitles apply only to the subtitle role", id)
	}
	if delivery == nil {
		if sub.GetFormat() != "" {
			return fmt.Errorf("media source: track %q: a burned-in subtitle track carries no format (language, role, forced only)", id)
		}
	}
	return nil
}

func validateDelivery(addressable corev1.Addressable, id string, d *corev1.TrackDelivery, ids map[string]struct{}) error {
	if d.GetCarriedIn() != "" {
		if d.GetCarriedIn() == id {
			return fmt.Errorf("media source: track %q: carried_in cannot reference itself", id)
		}
		if _, ok := ids[d.GetCarriedIn()]; !ok {
			return fmt.Errorf("media source: track %q: carried_in references unknown track %q", id, d.GetCarriedIn())
		}
	}
	if addressable == corev1.Addressable_ADDRESSABLE_PER_TRACK {
		if d.GetCarriedIn() != "" {
			return fmt.Errorf("media source: track %q: carried_in is not allowed in a PER_TRACK manifest", id)
		}
		if len(d.GetLocations()) == 0 {
			return fmt.Errorf("media source: track %q: a PER_TRACK manifest requires at least one location per track", id)
		}
		return nil
	}
	if len(d.GetLocations()) == 0 && d.GetCarriedIn() == "" {
		return fmt.Errorf("media source: track %q: not reachable: needs at least one location or carried_in", id)
	}
	return nil
}

// ValidateJob checks a Job (PLAN.md §9.1).
func ValidateJob(job *corev1.Job) error {
	if job == nil {
		return fmt.Errorf("job: nil")
	}
	if job.GetId() == "" {
		return fmt.Errorf("job: id is required")
	}
	if job.GetKind() == corev1.JobKind_JOB_KIND_UNSPECIFIED {
		return fmt.Errorf("job: kind is required")
	}
	if job.GetStatus() == corev1.JobStatus_JOB_STATUS_UNSPECIFIED {
		return fmt.Errorf("job: status is required")
	}
	if job.GetOwnerUserId() == "" {
		return fmt.Errorf("job: owner_user_id is required (a job's event audience is its owner; PLAN.md §9.2)")
	}
	if p := job.GetProgress(); p != nil && p.GetPercent() > 100 {
		return fmt.Errorf("job: progress percent must be at most 100")
	}
	if job.GetKind() == corev1.JobKind_JOB_KIND_DELIVERY {
		if err := validateDeliveryContext(job.GetDelivery()); err != nil {
			return err
		}
	}
	if job.GetStatus() == corev1.JobStatus_JOB_STATUS_AWAITING_ACTION {
		if err := validateAwaitingAction(job.GetAwaitingAction()); err != nil {
			return err
		}
	}
	if job.GetStatus() == corev1.JobStatus_JOB_STATUS_FAILED && job.GetError() == "" {
		return fmt.Errorf("job: error is required for FAILED jobs")
	}
	return nil
}

func validateDeliveryContext(d *corev1.DeliveryContext) error {
	if d == nil {
		return fmt.Errorf("job: delivery context is required for DELIVERY jobs (PLAN.md §2.3)")
	}
	if d.GetProvider() == "" || d.GetAccountId() == "" || d.GetMemberUserId() == "" {
		return fmt.Errorf("job: delivery context requires provider, account_id, and member_user_id")
	}
	return nil
}

func validateAwaitingAction(a *corev1.AwaitingAction) error {
	if a == nil {
		return fmt.Errorf("job: awaiting_action is required for AWAITING_ACTION jobs (PLAN.md §9.1)")
	}
	if a.GetActorUserId() == "" {
		return fmt.Errorf("job: awaiting_action carries who must act; actor_user_id is required")
	}
	if a.GetCommon() == corev1.ActionType_ACTION_TYPE_UNSPECIFIED && a.GetPassthrough() == nil {
		return fmt.Errorf("job: awaiting_action requires a common action or a passthrough prompt")
	}
	if p := a.GetPassthrough(); p != nil && (p.GetAdapter() == "" || p.GetDescriptor_() == "") {
		return fmt.Errorf("job: awaiting_action: a passthrough prompt requires adapter and descriptor")
	}
	return nil
}

// ValidateEventEnvelope checks an EventEnvelope (PLAN.md §9.2).
func ValidateEventEnvelope(e *corev1.EventEnvelope) error {
	if e == nil {
		return fmt.Errorf("event: nil")
	}
	if e.GetId() == "" {
		return fmt.Errorf("event: id is required")
	}
	if e.GetType() == corev1.EventType_EVENT_TYPE_UNSPECIFIED {
		return fmt.Errorf("event: type is required")
	}
	if e.GetAudience() == corev1.EventAudience_EVENT_AUDIENCE_UNSPECIFIED {
		return fmt.Errorf("event: audience is required")
	}
	switch e.GetAudience() {
	case corev1.EventAudience_EVENT_AUDIENCE_USER, corev1.EventAudience_EVENT_AUDIENCE_OWNER:
		if e.GetUserId() == "" {
			return fmt.Errorf("event: user_id is required for USER and OWNER audiences (PLAN.md §9.2)")
		}
	case corev1.EventAudience_EVENT_AUDIENCE_ACCOUNT:
		if e.GetAccountId() == "" {
			return fmt.Errorf("event: account_id is required for the ACCOUNT audience")
		}
	}
	switch e.GetType() {
	case corev1.EventType_EVENT_TYPE_JOB_STATUS:
		js := e.GetJobStatus()
		if js == nil {
			return fmt.Errorf("event: a job_status payload is required for JOB_STATUS events")
		}
		if js.GetJobId() == "" || js.GetStatus() == corev1.JobStatus_JOB_STATUS_UNSPECIFIED {
			return fmt.Errorf("event: a job_status payload requires job_id and status")
		}
	case corev1.EventType_EVENT_TYPE_MERGE_CONFLICT:
		mc := e.GetMergeConflict()
		if mc == nil {
			return fmt.Errorf("event: a merge_conflict payload is required for MERGE_CONFLICT events")
		}
		if mc.GetProvider() == "" || mc.GetProviderId() == "" || mc.GetEntryId() == "" {
			return fmt.Errorf("event: a merge_conflict payload requires provider, provider_id, and entry_id")
		}
	case corev1.EventType_EVENT_TYPE_PROVIDER_SWITCHED:
		ps := e.GetProviderSwitched()
		if ps == nil {
			return fmt.Errorf("event: a provider_switched payload is required for PROVIDER_SWITCHED events")
		}
		if ps.GetFromProvider() == "" || ps.GetToProvider() == "" {
			return fmt.Errorf("event: a provider_switched payload requires from_provider and to_provider")
		}
	case corev1.EventType_EVENT_TYPE_ACCOUNT_SESSION_LINKED,
		corev1.EventType_EVENT_TYPE_ACCOUNT_SESSION_EXPIRED,
		corev1.EventType_EVENT_TYPE_ACCOUNT_SESSION_REVOKED:
		as := e.GetAccountSession()
		if as == nil {
			return fmt.Errorf("event: an account_session payload is required for ACCOUNT_SESSION events")
		}
		if as.GetAccountId() == "" || as.GetProvider() == "" {
			return fmt.Errorf("event: an account_session payload requires account_id and provider")
		}
	case corev1.EventType_EVENT_TYPE_AVAILABILITY_CHANGED:
		av := e.GetAvailability()
		if av == nil {
			return fmt.Errorf("event: an availability payload is required for AVAILABILITY_CHANGED events")
		}
		if av.GetAccountId() == "" || av.GetProvider() == "" || av.GetEntryId() == "" {
			return fmt.Errorf("event: an availability payload requires account_id, provider, and entry_id")
		}
	}
	return nil
}

// ValidateTitleMetadata checks a TitleMetadata record (PLAN.md §5.2).
func ValidateTitleMetadata(tm *corev1.TitleMetadata) error {
	if tm == nil {
		return fmt.Errorf("title metadata: nil")
	}
	if tm.GetTitle() == "" {
		return fmt.Errorf("title metadata: title is required")
	}
	if tm.GetMovie() == nil && tm.GetSeries() == nil {
		return fmt.Errorf("title metadata: kind_specific (movie or series) is required")
	}
	return nil
}
