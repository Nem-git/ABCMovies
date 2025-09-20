package manifest

import (
	"io"
	"testing"

	"github.com/nem-git/abcmovies/internal/utils"
)

var normalManifests = []string{
	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/ad-insertion-testcase1.mpd",
	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/ad-insertion-testcase6-av1.mpd",
	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/ad-insertion-testcase6-av2.mpd",
	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/ad-insertion-testcase6-av5.mpd",
	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/admanager.xml",
	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/dash-testcases-5b-1-thomson.mpd",
	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/dashif-live-atoinf.mpd",
	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/dashif-low-latency.mpd",
	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/dolby-ac4.xml",
	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/f64-inf.mpd",
	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/multiple_supplementals.mpd",
	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/patch-location.mpd",
	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/st-sl.mpd",
	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/telenet-mid-ad-rolls.mpd",
	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/telestream-binary.xml",
	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/telestream-elements.xml",
	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/vod-aip-unif-streaming.mpd",
	"https://github.com/avishaycohen/mpd-parser/raw/refs/heads/main/manifests/bigBuckBunny-onDemend.mpd",
	"https://github.com/avishaycohen/mpd-parser/raw/refs/heads/main/manifests/bigBuckBunny-simple.mpd",
	"https://github.com/avishaycohen/mpd-parser/raw/refs/heads/main/manifests/bitmovin-sample.mpd",
	"https://github.com/avishaycohen/mpd-parser/raw/refs/heads/main/manifests/chunked-single-bitrate.mpd",
	"https://github.com/avishaycohen/mpd-parser/raw/refs/heads/main/manifests/client_manifest-720p-stereo-events.mpd",
	"https://github.com/avishaycohen/mpd-parser/raw/refs/heads/main/manifests/client_manifest-audio.mpd",
	"https://github.com/avishaycohen/mpd-parser/raw/refs/heads/main/manifests/client_manifest-ctv-events.mpd",
	"https://github.com/avishaycohen/mpd-parser/raw/refs/heads/main/manifests/client_manifest-events-multilang.mpd",
	"https://github.com/avishaycohen/mpd-parser/raw/refs/heads/main/manifests/client_manifest-events-nodvb.mpd",
	"https://github.com/avishaycohen/mpd-parser/raw/refs/heads/main/manifests/client_manifest-events.mpd",
	"https://github.com/avishaycohen/mpd-parser/raw/refs/heads/main/manifests/client_manifest-nointerlace-events.mpd",
	"https://github.com/avishaycohen/mpd-parser/raw/refs/heads/main/manifests/client_manifest-nosurround-ctv-multilang.mpd",
	"https://github.com/avishaycohen/mpd-parser/raw/refs/heads/main/manifests/client_manifest-pto_both-events.mpd",
	"https://github.com/avishaycohen/mpd-parser/raw/refs/heads/main/manifests/client_manifest.mpd",
	"https://github.com/avishaycohen/mpd-parser/raw/refs/heads/main/manifests/multirate.mpd",
	"https://dash.akamaized.net/dash264/TestCases/2c/qualcomm/1/MultiResMPEG2.mpd",
	"https://livesim.dashif.org/dash/vod/testpic_2s/multi_subs.mpd",
	"http://media.axprod.net/TestVectors/v6-Clear/Manifest_1080p.mpd",
	"https://a38avoddashs3ww-a.akamaihd.net/ondemand/iad_2/8e91/f2f2/ec5a/430f-bd7a-0779f4a0189d/685cda75-609c-41c1-86bb-688f4cdb5521_corrected.mpd",
	"http://playready.directtaps.net/smoothstreaming/SSWSS720H264/SuperSpeedway_720.ism/Manifest",
	"https://azclwds01.akamaized.net/4e8f6858-5d05-4e28-83ab-48c7a2b259e1/XVuosg_tab_hd.ism/Manifest",
	"https://media.axprod.net/TestVectors/v7-Clear/Manifest_1080p.mpd",
	"https://cmafref.akamaized.net/cmaf/live-ull/2006350/akambr/out.mpd",
	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/mediapackage.xml",
}

var protectedManifests = []string{
	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/a2d-tv.mpd",
	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/orange.xml",

	"https://a38avoddashs3ww-a.akamaihd.net/ondemand/iad_2/8e91/f2f2/ec5a/430f-bd7a-0779f4a0189d/685cda75-609c-41c1-86bb-688f4cdb5521_corrected.mpd",

	"https://cbcrcott-aws-toutv.akamaized.net/out/v1/e71b9f2ee8684145982294c54e2311ed/97c7a58d11d84ea78801a32f293d0a21/27f2eb30c8fb43f99ba46fee14ce2d37/index-multi-drm.mpd?pckgrp=bd19b98e3f6f49156464835f3aa1e8bb&ewid=83314&filter=3000&EIA608ClosedCaptions=true&lang=fr",
	"https://cbcrcott-aws-toutv.akamaized.net/out/v1/e71b9f2ee8684145982294c54e2311ed/97c7a58d11d84ea78801a32f293d0a21/27f2eb30c8fb43f99ba46fee14ce2d37/index-multi-drm.mpd?pckgrp=bd19b98e3f6f49156464835f3aa1e8bb&ewid=83314&filter=7000&EIA608ClosedCaptions=true&lang=fr",
}

// Here, the manifests are invalid, so supposed to break
var breakingManifests = []string{
	// Here, it breaks because there are no init or media segments in some adaptation sets and representations
	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/avod-mediatailor.mpd",                                  // no init or media in segmenttemplate
	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/aws.xml",                                               // no init or media in segmenttemplate
	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/example_G22.mpd",                                       // no init or media in segmenttemplate
	"https://github.com/avishaycohen/mpd-parser/raw/refs/heads/main/manifests/aws-media-tailor-vod-personalized-response-manifest.mpd", // no init or media in segmenttemplate

	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/jurassic-compact-5975.mpd", // couldn't parse xml using xpath

	"https://bitmovin-a.akamaihd.net/content/art-of-motion_drm/mpds/11331.mpd", // XML syntax error on line 9: element <P> closed by </BODY>

	"https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/incomplete.mpd", // incomplete
}

func TestNormalManifests(t *testing.T) {
	for _, url := range normalManifests {
		// Send request to get Dash Manifest
		body, err := utils.Get(url, nil, nil)
		if err != nil {
			t.Errorf(`utils.Get(url, nil, nil) = %q, %v`, url, err)
		}
		defer body.Close()

		b, err := io.ReadAll(body)
		if err != nil {
			t.Errorf(`io.ReadAll(body) = %q, %v`, url, err)
		}

		mpd, err := Get(url, string(b))
		if err != nil {
			t.Errorf(`Get(%v, string(body)) = %q, %v`, url, mpd, err)
		}
	}
}

func TestProtectedManifests(t *testing.T) {
	for _, url := range protectedManifests {
		// Send request to get Dash Manifest
		body, err := utils.Get(url, nil, nil)
		if err != nil {
			t.Errorf(`utils.Get(url, nil, nil) = %q, %v`, url, err)
		}
		defer body.Close()

		b, err := io.ReadAll(body)
		if err != nil {
			t.Errorf(`io.ReadAll(body) = %q, %v`, url, err)
		}

		mpd, err := Get(url, string(b))
		if err != nil {
			t.Errorf(`Get(%v, string(body)) = %q, %v`, url, mpd, err)
		}
	}
}

func TestBreakingManifests(t *testing.T) {
	for _, url := range breakingManifests {
		// Send request to get Dash Manifest
		body, err := utils.Get(url, nil, nil)
		if err != nil {
			t.Errorf(`utils.Get(url, nil, nil) = %q, %v`, url, err)
		}
		defer body.Close()

		b, err := io.ReadAll(body)
		if err != nil {
			t.Errorf(`io.ReadAll(body) = %q, %v`, url, err)
		}

		_, err = Get(url, string(b))
		if err == nil {
			t.Errorf(`Get(%v, string(body)) = %q`, url, err)
		}
	}
}
