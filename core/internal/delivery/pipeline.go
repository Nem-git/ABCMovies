package delivery

import (
	"fmt"
	"regexp"
	"strings"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

// StepKind names one byte-transform stage in a delivery pipeline (§6.3).
// Pipelines are the ordered sequence of these steps one session runs.
type StepKind string

const (
	// StepPassthrough delivers native bytes unchanged.
	StepPassthrough StepKind = "passthrough"
	// StepDecrypt clears a DRM-encrypted source before any other transform
	// (§6.2 "DRM before anything", §6.6). v1 has no decryption engine and the
	// manifest carries no encryption signal, so SelectPipeline never emits it;
	// it is reserved here so the chain model stays complete (§3.4 additive).
	StepDecrypt StepKind = "decrypt"
	// StepRemux re-containers a stream without re-encoding; it also selects
	// and drops streams (e.g. removing subtitles/audio) while re-containerizing
	// (§6.3). v1 implements a container-copy route.
	StepRemux StepKind = "remux"
	// StepTranscode encodes from the best qualifying native track to a target;
	// never upscales (fallback-only, §6.3). v1 ships no encoder, so it is
	// declined loudly and logged, never faked.
	StepTranscode StepKind = "transcode"
	// StepCompose assembles the selected, finalized tracks plus the target
	// container into one deliverable. Download-only.
	StepCompose StepKind = "compose"
	// StepRecord captures a live source, gap-handled, stop → finalize. A later
	// milestone; declined loudly in v1.
	StepRecord StepKind = "record"
)

// Plan is the ordered step chain one delivery session runs (§6.3).
// SelectPipeline builds it from the goal and source; Start records it on the
// session (the decision record and the seed of the resume key, §6.5) and
// validates every step is executable, declining unsupported ones loudly
// (§2.5: reject, never downgrade).
type Plan struct {
	Steps []Step
}

// Step is one stage of a plan. Inputs names the prior steps' outputs this
// step consumes (DAG edges); when empty, the step implicitly consumes its
// immediate predecessor's output — the identity case of a linear chain.
type Step struct {
	// Name optionally labels this step's output so a later step can address
	// it in Inputs (a required name for any step others fan in from).
	Name   string
	Kind   StepKind
	Inputs []string
	// Params holds the typed knobs for this step; only the sub-struct matching
	// Kind is set (a discriminated union), the rest are nil.
	Params StepParams
}

// StepParams is the per-step configuration. It is a discriminated union: the
// sub-struct matching the step's Kind is set; the others stay nil. It is typed
// (not a string bag) so a mis-wired step fails loudly at plan time rather than
// misbehaving at execution.
type StepParams struct {
	Passthrough *PassthroughParams
	Decrypt     *DecryptParams
	Remux       *RemuxParams
	Transcode   *TranscodeParams
	Compose     *ComposeParams
	Record      *RecordParams
}

// PassthroughParams carries no knobs; passthrough hands bytes on unchanged.
type PassthroughParams struct{}

// DecryptParams names the license/key material needed to clear a stream (§6.6).
type DecryptParams struct {
	// KeyRef identifies the vaulted key/license session to use.
	KeyRef string
}

// RemuxParams controls a re-container step, including which streams survive.
type RemuxParams struct {
	// Container is the target container extension (e.g. "mkv", "mp4").
	Container string
	// Keep is a coarse keep/drop filter by media type. Removing all subtitles
	// (Subtitle=false) is expressed here (PLAN.md §6.3: remux selects/drops
	// streams). An empty Keep selects everything.
	Keep TrackFilter
	// KeepTracks, when non-empty, names exact track ids to keep (any track not
	// listed is dropped). It refines or overrides Keep.
	KeepTracks []string
}

// TrackFilter is a coarse keep/drop switch by media type for a remux step.
type TrackFilter struct {
	Video    bool
	Audio    bool
	Subtitle bool
}

// TranscodeParams describes one or more encode targets (§6.3 targets).
type TranscodeParams struct {
	Profiles []EncodeProfile
}

// EncodeProfile is one encode target: a codec and a resolution/bitrate bound.
type EncodeProfile struct {
	Codec      string
	Height     int
	MaxBitrate string
}

// ComposeParams controls the final assemble-into-file step (download-only).
type ComposeParams struct {
	// Container is the target container of the composed deliverable.
	Container string
	// AttachMetadata optionally embeds metadata cache fields (§5.2) into the
	// deliverable; off by default, inheriting enrichment's opt-in rules (§6.4).
	AttachMetadata bool
}

// RecordParams carries no knobs; recording appends live bytes to the sink
// (§6.4 recording-is-a-sink). Reserved for a later milestone.
type RecordParams struct{}

// SelectPipeline chooses the ordered step chain for a session, by §6.3
// precedence (user request > instance config > delivery goal > engine default).
// It never fabricates a step it cannot honour: a selection that would need
// real byte-level re-encoding or muxing is rejected explicitly rather than
// silently downgraded (PLAN.md §2.5). v1 selects only executable steps —
// passthrough, container-copy remux (with stream selection), and compose.
func SelectPipeline(goal Goal, src *corev1.MediaSource, container string) (Plan, error) {
	if src == nil {
		return Plan{}, fmt.Errorf("pipeline: nil manifest")
	}
	switch goal {
	case GoalPlay:
		// A play serves the native manifest to the player. A per-track source
		// is always passthrough. A whole-mux source is passthrough unless the
		// sink explicitly asks for a different container, in which case we
		// re-container it (v1 container copy).
		if src.GetAddressable() == corev1.Addressable_ADDRESSABLE_PER_TRACK {
			return Plan{Steps: []Step{{Kind: StepPassthrough}}}, nil
		}
		if container == "" {
			return Plan{Steps: []Step{{Kind: StepPassthrough}}}, nil
		}
		return Plan{Steps: []Step{{Kind: StepRemux, Params: StepParams{Remux: &RemuxParams{Container: container}}}}}, nil
	case GoalDownload:
		// A download always lands as a named container (compose). A whole-mux
		// source is remux'd to the requested container (or its native one when
		// absent) and composed — no re-encode. A per-track source composed
		// into a single container needs real muxing, which v1 does not do;
		// reported, never guessed.
		if src.GetAddressable() == corev1.Addressable_ADDRESSABLE_PER_TRACK {
			if container == "" {
				return Plan{}, fmt.Errorf("pipeline: per-track download without a container cannot be composed without muxing (v1)")
			}
			return Plan{}, fmt.Errorf("pipeline: per-track download to container %q needs real muxing, not in v1", container)
		}
		return Plan{Steps: []Step{
			{Kind: StepRemux, Params: StepParams{Remux: &RemuxParams{Container: container}}},
			{Kind: StepCompose, Params: StepParams{Compose: &ComposeParams{Container: container}}},
		}}, nil
	default:
		return Plan{}, fmt.Errorf("pipeline: unknown goal %q", goal)
	}
}

// executableKinds is the set of steps v1 can genuinely run (no re-encode, no
// DRM decrypt, no live record). Anything else is declined loudly and logged,
// never faked into a passthrough (PLAN.md §2.5).
var executableKinds = map[StepKind]bool{
	StepPassthrough: true,
	StepRemux:       true,
	StepCompose:     true,
}

// notExecutableReason names the v1 reason a step kind is declined, for the
// honest log-and-fail message.
func notExecutableReason(k StepKind) string {
	switch k {
	case StepDecrypt:
		return "DRM decrypt is not implemented in v1 (requires §6.6 key/license handling)"
	case StepTranscode:
		return "transcode is not implemented in v1 (no encoder; the native-first, encode-only-as-fallback rule of §6.3 is unimplemented)"
	case StepRecord:
		return "live record is a later milestone and is not implemented in v1"
	default:
		return fmt.Sprintf("step %q is not executable in v1", k)
	}
}

// ValidatePlan asserts every step in the plan is executable by this v1 engine
// and that the plan is non-empty. It is the honest-decline gate: a plan that
// contains a step the engine cannot run is rejected, with the offending step
// and kind surfaced — never silently downgraded (PLAN.md §2.5). The caller
// (Start) maps this onto a loud session failure that logs the full plan.
func ValidatePlan(p Plan) error {
	if len(p.Steps) == 0 {
		return fmt.Errorf("plan: no steps")
	}
	for _, s := range p.Steps {
		if !executableKinds[s.Kind] {
			return fmt.Errorf("plan: step %q (%s): %s", s.Name, s.Kind, notExecutableReason(s.Kind))
		}
	}
	return nil
}

// Lineage returns a compact, human-readable rendering of the step chain
// (e.g. "remux → compose") for logging and decision records.
func Lineage(p Plan) string {
	out := make([]string, 0, len(p.Steps))
	for _, s := range p.Steps {
		n := string(s.Kind)
		if s.Name != "" {
			n = s.Name + ":" + n
		}
		out = append(out, n)
	}
	return strings.Join(out, " → ")
}

// containerSuffixPattern matches characters that are illegal or dangerous in a
// final path name across the platforms we care about (TECHNICAL-DECISIONS.md
// §1.15: "characters illegal in path names are stripped"). Purely cosmetic
// characters that are legal in a file name are preserved.
var containerSuffixPattern = regexp.MustCompile(`[/\\:*?"<>|\x00]+`)

// sanitizeName strips characters illegal in path names and collapses runs of
// separators, per the output naming contract (TECHNICAL-DECISIONS.md §1.15).
func sanitizeName(s string) string {
	s = containerSuffixPattern.ReplaceAllString(s, " ")
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}
