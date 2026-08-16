package mp4

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/aac"
	"github.com/Eyevinn/mp4ff/avc"
	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/asticode/go-astits"

	"github.com/nem-git/abcmovies/internal/proxy/parser"
	"github.com/nem-git/abcmovies/internal/stream"
)

// tsPES is a parsed PES packet from an MPEG-TS elementary stream.
type tsPES struct {
	data     []byte
	streamID uint8
	pts      uint64 // 90 kHz
	dts      uint64 // 90 kHz
}

// tsSample is one output sample (video: length-prefixed NALUs, audio: raw AAC).
type tsSample struct {
	data []byte
	pts  uint64 // 90 kHz
	dts  uint64 // 90 kHz
	sync bool
}

// sampleTime is the resolved timing of one sample in its track timescale.
type sampleTime struct {
	decode uint32
	dur    uint32
	cto    int32
}

// segResult holds the demuxed samples for one input TS segment.
type segResult struct {
	video  []tsSample
	audio  []tsSample
	vTimes []sampleTime
	aTimes []sampleTime
}

// convertTSToMP4 converts HLS MPEG-TS segments into a single fragmented MP4.
func (c *Converter) convertTSToMP4(ctx context.Context, src *stream.Locator, videoPl, audioPl *parser.MediaPlaylist, videoBase, audioBase string, w io.Writer) error {
	var results []segResult
	var sps, pps [][]byte
	audioSampleRate := 0

	for i, seg := range videoPl.Segments {
		if seg == nil || seg.URI == "" {
			continue
		}
		if seg.Limit > 0 {
			return fmt.Errorf("convert: byte-range TS segments are not supported (%s)", seg.URI)
		}
		body, _, err := c.fetcher.Fetch(ctx, parser.ResolveURL(videoBase, seg.URI), src.Headers, src.Query)
		if err != nil {
			return fmt.Errorf("convert: fetching TS segment %s: %w", seg.URI, err)
		}
		buckets, esTypes, derr := demuxTSFile(ctx, body)
		body.Close()
		if derr != nil {
			return fmt.Errorf("convert: demuxing TS segment %s: %w", seg.URI, derr)
		}
		vPID, aPID, ok := pickVideoAudioPIDs(buckets, esTypes)
		if !ok {
			return fmt.Errorf("convert: no video stream found in TS segment %s", seg.URI)
		}

		var res segResult
		res.video, sps, pps = videoSamplesFromPES(buckets[vPID])
		if aPID != 0 && len(buckets[aPID]) > 0 {
			res.audio, audioSampleRate, _ = audioSamplesFromPES(buckets[aPID])
		} else if audioPl != nil && i < len(audioPl.Segments) && audioPl.Segments[i] != nil && audioPl.Segments[i].URI != "" {
			aseg := audioPl.Segments[i]
			ab, _, ferr := c.fetcher.Fetch(ctx, parser.ResolveURL(audioBase, aseg.URI), src.Headers, src.Query)
			if ferr != nil {
				return fmt.Errorf("convert: fetching audio TS segment %s: %w", aseg.URI, ferr)
			}
			abuckets, aesTypes, aderr := demuxTSFile(ctx, ab)
			ab.Close()
			if aderr != nil {
				return fmt.Errorf("convert: demuxing audio TS segment %s: %w", aseg.URI, aderr)
			}
			if aPID2, _, ok2 := pickVideoAudioPIDs(abuckets, aesTypes); ok2 && aPID2 != 0 {
				res.audio, audioSampleRate, _ = audioSamplesFromPES(abuckets[aPID2])
			}
		}
		if len(res.video) > 0 || len(res.audio) > 0 {
			results = append(results, res)
		}
	}

	if len(results) == 0 {
		return fmt.Errorf("convert: no playable media found in HLS stream")
	}

	init, err := buildTSInit(sps, pps, audioSampleRate)
	if err != nil {
		return err
	}
	videoTrackID := init.Moov.Trak.Tkhd.TrackID
	audioTrackID := uint32(0)
	if len(init.Moov.Traks) > 1 {
		audioTrackID = init.Moov.Traks[1].Tkhd.TrackID
	}
	if err := init.Encode(w); err != nil {
		return fmt.Errorf("convert: encoding TS-derived init segment: %w", err)
	}

	if err := resolveVideoTiming(results); err != nil {
		return err
	}
	resolveAudioTiming(results, audioSampleRate)

	seq := uint32(1)
	for _, res := range results {
		var trackIDs []uint32
		if len(res.video) > 0 {
			trackIDs = append(trackIDs, videoTrackID)
		}
		if len(res.audio) > 0 && audioTrackID != 0 {
			trackIDs = append(trackIDs, audioTrackID)
		}
		if len(trackIDs) == 0 {
			continue
		}
		frag, err := mp4.CreateMultiTrackFragment(seq, trackIDs)
		if err != nil {
			return fmt.Errorf("convert: creating TS-derived fragment: %w", err)
		}
		seg := mp4.NewMediaSegmentWithoutStyp()
		seg.AddFragment(frag)

		for si, s := range res.video {
			fs := toFullSample(s, res.vTimes[si], videoTrackID)
			if err := frag.AddFullSampleToTrack(fs, videoTrackID); err != nil {
				return fmt.Errorf("convert: adding video sample to fragment: %w", err)
			}
		}
		for si, s := range res.audio {
			fs := toFullSample(s, res.aTimes[si], audioTrackID)
			if err := frag.AddFullSampleToTrack(fs, audioTrackID); err != nil {
				return fmt.Errorf("convert: adding audio sample to fragment: %w", err)
			}
		}
		if err := seg.Encode(w); err != nil {
			return fmt.Errorf("convert: encoding TS-derived media segment: %w", err)
		}
		seq++
	}
	return nil
}

// toFullSample converts a tsSample into an mp4ff FullSample.
func toFullSample(s tsSample, t sampleTime, trackID uint32) mp4.FullSample {
	flags := mp4.NonSyncSampleFlags
	if s.sync {
		flags = mp4.SyncSampleFlags
	}
	return mp4.FullSample{
		Sample: mp4.Sample{
			Flags:                 flags,
			Dur:                   t.dur,
			Size:                  uint32(len(s.data)),
			CompositionTimeOffset: t.cto,
		},
		DecodeTime: uint64(t.decode),
		Data:       s.data,
	}
}

// buildTSInit builds the init segment for a TS-derived MP4.
func buildTSInit(sps, pps [][]byte, audioSampleRate int) (*mp4.InitSegment, error) {
	init := mp4.CreateEmptyInit()
	if len(sps) > 0 && len(pps) > 0 {
		trak := init.AddEmptyTrack(90000, "video", "und")
		if err := trak.SetAVCDescriptor("avc1", sps, pps, true); err != nil {
			return nil, fmt.Errorf("convert: setting video descriptor: %w", err)
		}
	}
	if audioSampleRate > 0 {
		trak := init.AddEmptyTrack(uint32(audioSampleRate), "audio", "und")
		if err := trak.SetAACDescriptor(aac.AAClc, audioSampleRate); err != nil {
			return nil, fmt.Errorf("convert: setting audio descriptor: %w", err)
		}
	}
	if init.Moov.Traks == nil || len(init.Moov.Traks) == 0 {
		return nil, fmt.Errorf("convert: TS stream contains no H.264 video or AAC audio")
	}
	return init, nil
}

// resolveVideoTiming computes absolute decode times, durations and composition
// offsets for video samples in the 90000 Hz video timescale.
func resolveVideoTiming(results []segResult) error {
	var decode []uint64
	var minDecode uint64
	first := true
	for _, res := range results {
		for _, s := range res.video {
			decode = append(decode, s.dts)
			if first || s.dts < minDecode {
				minDecode = s.dts
				first = false
			}
		}
	}
	if len(decode) == 0 {
		return fmt.Errorf("convert: no video samples found")
	}
	idx := 0
	var prevDur uint32
	for ri := range results {
		results[ri].vTimes = make([]sampleTime, len(results[ri].video))
		for si, s := range results[ri].video {
			var dur uint32
			if idx+1 < len(decode) && decode[idx+1] > decode[idx] {
				dur = uint32(decode[idx+1] - decode[idx])
			}
			if dur == 0 {
				dur = prevDur
			}
			if dur == 0 {
				dur = 3000 // default 1/30 s at 90 kHz
			}
			results[ri].vTimes[si] = sampleTime{
				decode: uint32(decode[idx] - minDecode),
				dur:    dur,
				cto:    int32(int64(s.pts) - int64(s.dts)),
			}
			prevDur = dur
			idx++
		}
	}
	return nil
}

// resolveAudioTiming assigns decode times to audio samples in the audio
// timescale, anchored to the video timeline. dur is always 1024 (AAC-LC).
func resolveAudioTiming(results []segResult, sampleRate int) {
	if sampleRate == 0 {
		return
	}
	var firstPTS uint64
	haveFirst := false
	count := 0
	for _, res := range results {
		for _, s := range res.audio {
			if !haveFirst {
				firstPTS = s.pts
				haveFirst = true
			}
			count++
		}
	}
	base := int64(0)
	if haveFirst {
		base = int64(firstPTS) * int64(sampleRate) / 90000
		if base < 0 {
			base = 0
		}
	}
	frame := 0
	for ri := range results {
		results[ri].aTimes = make([]sampleTime, len(results[ri].audio))
		for si := range results[ri].audio {
			results[ri].aTimes[si] = sampleTime{
				decode: uint32(base + int64(frame)*1024),
				dur:    1024,
				cto:    0,
			}
			frame++
		}
	}
}

// demuxTSFile parses an MPEG-TS and groups PES packets by PID.
func demuxTSFile(ctx context.Context, r io.Reader) (map[uint16][]tsPES, map[uint16]astits.StreamType, error) {
	d := astits.NewDemuxer(ctx, r)
	buckets := map[uint16][]tsPES{}
	esTypes := map[uint16]astits.StreamType{}
	for {
		data, err := d.NextData()
		if err == astits.ErrNoMorePackets {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		if data.PMT != nil {
			for _, es := range data.PMT.ElementaryStreams {
				if es != nil {
					esTypes[es.ElementaryPID] = es.StreamType
				}
			}
			continue
		}
		if data.PES != nil && data.PES.Header != nil {
			p := tsPES{data: data.PES.Data, streamID: data.PES.Header.StreamID}
			if oh := data.PES.Header.OptionalHeader; oh != nil {
				if oh.PTS != nil {
					p.pts = uint64(oh.PTS.Base)
					p.dts = p.pts
				}
				if oh.DTS != nil {
					p.dts = uint64(oh.DTS.Base)
				}
			}
			buckets[data.PID] = append(buckets[data.PID], p)
		}
	}
	return buckets, esTypes, nil
}

// pickVideoAudioPIDs selects the H.264 video and AAC audio PIDs, preferring PMT
// stream types and falling back to PES stream IDs.
func pickVideoAudioPIDs(buckets map[uint16][]tsPES, esTypes map[uint16]astits.StreamType) (videoPID, audioPID uint16, ok bool) {
	for pid, st := range esTypes {
		switch st {
		case astits.StreamTypeH264Video:
			videoPID = pid
		case astits.StreamTypeAACAudio: // 0x0f == StreamTypeADTS
			if audioPID == 0 {
				audioPID = pid
			}
		}
	}
	if videoPID != 0 {
		return videoPID, audioPID, true
	}
	for pid, pes := range buckets {
		for _, p := range pes {
			switch {
			case p.streamID >= 0xe0 && p.streamID <= 0xef:
				if videoPID == 0 {
					videoPID = pid
				}
			case p.streamID >= 0xc0 && p.streamID <= 0xdf:
				if audioPID == 0 {
					audioPID = pid
				}
			}
			if videoPID != 0 && audioPID != 0 {
				return videoPID, audioPID, true
			}
		}
	}
	return videoPID, audioPID, videoPID != 0
}

// videoSamplesFromPES converts video PES packets into length-prefixed samples,
// splitting access units at AUD NALUs and extracting SPS/PPS.
func videoSamplesFromPES(pes []tsPES) (samples []tsSample, sps, pps [][]byte) {
	var all []byte
	for _, p := range pes {
		all = append(all, p.data...)
	}
	sps, pps = avc.GetParameterSetsFromByteStream(all)
	for _, au := range splitAccessUnits(pes) {
		data := avc.ConvertByteStreamToNaluSample(au.annexb)
		if len(data) == 0 {
			continue
		}
		samples = append(samples, tsSample{data: data, pts: au.pts, dts: au.dts, sync: isIDR(au.annexb)})
	}
	return samples, sps, pps
}

// tsAU is one access unit in Annex-B form.
type tsAU struct {
	annexb []byte
	pts    uint64
	dts    uint64
}

// splitAccessUnits groups PES packets into access units, splitting within a
// PES payload at AUD NALUs when present.
func splitAccessUnits(pes []tsPES) []tsAU {
	var aus []tsAU
	for _, p := range pes {
		if chunks := splitAtAUD(p.data); chunks != nil {
			for _, c := range chunks {
				aus = append(aus, tsAU{annexb: c, pts: p.pts, dts: p.dts})
			}
			continue
		}
		if len(p.data) > 0 {
			aus = append(aus, tsAU{annexb: p.data, pts: p.pts, dts: p.dts})
		}
	}
	return aus
}

const naluAUD = 9

// splitAtAUD splits an Annex-B byte stream at each AUD NALU. Returns nil when
// the stream contains no AUD.
func splitAtAUD(data []byte) [][]byte {
	offs := naluOffsets(data)
	var audIdx []int
	for _, off := range offs {
		if off < len(data) && data[off]&0x1F == naluAUD {
			audIdx = append(audIdx, off)
		}
	}
	if len(audIdx) == 0 {
		return nil
	}
	var chunks [][]byte
	prev := 0
	for _, aoff := range audIdx {
		chunks = append(chunks, data[prev:aoff])
		prev = aoff
	}
	chunks = append(chunks, data[prev:])
	if len(chunks[0]) > 0 && len(chunks) > 1 {
		chunks[1] = append(append([]byte{}, chunks[0]...), chunks[1]...)
		chunks = chunks[1:]
	}
	var out [][]byte
	for _, c := range chunks {
		if len(c) > 0 {
			out = append(out, c)
		}
	}
	return out
}

// naluOffsets returns the byte offsets of the NALU headers (just after the
// start code) in an Annex-B byte stream.
func naluOffsets(data []byte) []int {
	var offsets []int
	i := 0
	n := len(data)
	for i < n {
		switch {
		case i+3 < n && data[i] == 0 && data[i+1] == 0 && data[i+2] == 1:
			offsets = append(offsets, i+3)
			i += 3
		case i+4 < n && data[i] == 0 && data[i+1] == 0 && data[i+2] == 0 && data[i+3] == 1:
			offsets = append(offsets, i+4)
			i += 4
		default:
			i++
		}
	}
	return offsets
}

// isIDR reports whether an access unit starts with an IDR slice (NAL type 5).
func isIDR(annexb []byte) bool {
	for _, off := range naluOffsets(annexb) {
		if off >= len(annexb) {
			continue
		}
		switch annexb[off] & 0x1F {
		case 5:
			return true
		case 1:
			return false
		}
	}
	return false
}

// audioSamplesFromPES splits AAC ADTS PES payloads into raw (header-less)
// frames and reports the sample rate.
func audioSamplesFromPES(pes []tsPES) (samples []tsSample, sampleRate int, err error) {
	var pts uint64
	for _, p := range pes {
		pts = p.pts
		data := p.data
		for len(data) > 0 {
			hdr, offset, aerr := aac.DecodeADTSHeader(bytes.NewReader(data))
			if aerr != nil {
				break
			}
			if offset >= len(data) || int(hdr.HeaderLength) > len(data)-offset {
				break
			}
			frameLen := int(hdr.HeaderLength) + int(hdr.PayloadLength)
			if frameLen > len(data)-offset || frameLen <= int(hdr.HeaderLength) {
				break
			}
			raw := data[offset+int(hdr.HeaderLength) : offset+frameLen]
			samples = append(samples, tsSample{data: append([]byte(nil), raw...), pts: pts, dts: pts, sync: true})
			if sampleRate == 0 {
				sampleRate = int(hdr.Frequency())
			}
			data = data[offset+frameLen:]
		}
	}
	return samples, sampleRate, nil
}
