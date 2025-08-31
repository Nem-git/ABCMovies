import urllib
import base64
import os
import requests
import re
from lxml import etree
from mpd_parser.parser import Parser
from mpd_parser.models.composite_tags import MPD
from uuid import uuid4

from src.models.models import DashManifestModificationRequest
from src.models.models import DashManifestModificationResponse


class ManifestModifier:

    mpd_url: str
    mpd_content: str

    base_url: str
    mpd_object: Parser

    def get_modified_mpd(
        self,
        request: DashManifestModificationRequest,
        response: DashManifestModificationResponse,
    ):
        try:
            self.mpd_content = request.content
            self.mpd_url = request.url
            # Sets the base url from the manifest
            self.mpd_object = Parser.from_string(self.mpd_content)
            self._set_initial_base_url()

            self._parse_xml()

            self.mpd_content = Parser.to_string(self.mpd_object)

        except Exception as e:
            response.error = str(e)
            response.content = ""
            return

        try:
            # Apply XML modifications (cleaning up DRM, etc.)
            self.mpd_content = self._apply_lxml_modifications(self.mpd_content)

            response.content = str(self.mpd_content)

        except Exception as e:
            response.error = str(e)
            response.content = ""

    def _set_initial_base_url(self):
        # This gives the Manifest url WITHOUT the manifest's filename
        # self.base_url = self._join_url(self.mpd_url, "../")
        self.base_url = self.mpd_url

    def _extend_base_url(self, element: Parser, original_base_url: str) -> str:

        if len(element.base_urls) > 0:
            return self._join_url(original_base_url, element.base_urls[0].text)

        return original_base_url

    def _join_url(self, base_url, new_part):
        return urllib.parse.urljoin(base_url, new_part)

    def _apply_lxml_modifications(self, mpd_xml_str: str) -> str:
        parser = etree.XMLParser(remove_blank_text=True, ns_clean=True, recover=True)
        root = etree.fromstring(mpd_xml_str.encode("utf-8"), parser)

        # Remove DRM-related elements
        root = self._remove_base_url(root)
        root = self._remove_content_protection(root)
        root = self._remove_events(root)
        root = self._remove_drm_namespaces(root)

        return etree.tostring(root, encoding="unicode", pretty_print=True)

    def _remove_drm_namespaces(self, root):
        mpd_content = etree.tostring(root, encoding="unicode")

        # Remove DRM namespaces and associated elements
        mpd_content = re.sub(r'\sxmlns:mspr="urn:microsoft:playready"', "", mpd_content)
        mpd_content = re.sub(r'\sxmlns:cenc="urn:mpeg:cenc:2013"', "", mpd_content)
        mpd_content = re.sub(r'\smspr:[^\s=]+="[^"]*"', "", mpd_content)
        mpd_content = re.sub(
            r"<mspr:[^>]+>[^<]*</mspr:[^>]+>", "", mpd_content, flags=re.DOTALL
        )

        return etree.fromstring(mpd_content.encode("utf-8"))

    def _remove_content_protection(self, root):
        # Remove <ContentProtection> tags
        cps = root.xpath(
            ".//mpd:ContentProtection",
            namespaces={"mpd": "urn:mpeg:dash:schema:mpd:2011"},
        )
        for cp in cps:
            parent = cp.getparent()
            if len(parent):
                parent.remove(cp)

        return root

    def _remove_events(self, root):
        # Remove <EventStream> tags
        cps = root.xpath(
            ".//scte:EventStream",
            namespaces={"scte": "urn:scte:scte35:2013:xml"},
        )
        for cp in cps:
            parent = cp.getparent()
            if len(parent):
                parent.remove(cp)

        return root

    def _remove_base_url(self, root):
        # Remove <BaseURL> tags
        cps = root.xpath(
            ".//mpd:BaseURL",
            namespaces={"mpd": "urn:mpeg:dash:schema:mpd:2011"},
        )
        for cp in cps:
            parent = cp.getparent()
            if len(parent):
                parent.remove(cp)

        return root

    def _parse_xml(self):
        mpd_base_url = self._extend_base_url(self.mpd_object, self.base_url)

        for period in self.mpd_object.periods:
            period_base_url = self._extend_base_url(period, mpd_base_url)

            for adaptation_set in period.adaptation_sets:
                adaptation_set_base_url = self._extend_base_url(
                    adaptation_set, period_base_url
                )
                self._parse_segment_template(adaptation_set, adaptation_set_base_url)

                for representation in adaptation_set.representations:
                    representation_base_url = self._extend_base_url(
                        representation, adaptation_set_base_url
                    )
                    self._parse_segment_template(
                        representation, representation_base_url
                    )

    def _parse_segment_template(self, parent, base_url=""):
        if not parent.segment_template:
            return

        segment_template = parent.segment_template

        if segment_template.initialization:
            init_media_id = self._get_unique_id()

            init_url = self._join_url(base_url, segment_template.initialization)
            formatted_init_path = self._get_formatted_path(init_url)

            media_url = self._join_url(base_url, segment_template.media)
            formatted_media_path = self._get_formatted_path(media_url)

            # Init Url also needs to know it's own checksum
            # Because of changing values in the url, like $RepresentationId$
            # it would be impossible to link the init with the media
            init_url = self._construct_segment_url(
                "dash/init", formatted_init_path, init_media_id
            )
            media_url = self._construct_segment_url(
                "dash/media", formatted_media_path, init_media_id
            )

            # Replace URLs in the segment template
            self._replace_init_url(segment_template, init_url)
            self._replace_media_url(segment_template, media_url)

        elif segment_template.media:
            media_url = self._join_url(base_url, segment_template.media)
            formatted_media_path = self._get_formatted_path(media_url)

            media_url = self._construct_segment_url("dash/media", formatted_media_path)

            # Replace URLs in the segment template
            self._replace_media_url(segment_template, media_url)

    def _get_formatted_path(self, url: str) -> str:
        split_url = urllib.parse.urlsplit(url)

        # This creates a link that looks like https/rcatoutv.ca/test
        new_path = "/".join([split_url.scheme, split_url.netloc]) + split_url.path

        if split_url.query:
            new_path += "?" + split_url.query

        if split_url.fragment:
            new_path += "#" + split_url.fragment

        return new_path

    def _construct_segment_url(
        self, segment_type: str, formatted_path: str, init_media_id: str = ""
    ) -> str:
        return os.path.join(segment_type, init_media_id, formatted_path)

    def _get_unique_id(self, text: str = "") -> str:
        return str(uuid4())

    def _get_last_url_part(self, url: str) -> str:
        # Removes trailing / and returns the last part of the URL (usually filename, ?q=params, #frags etc)
        return url.rstrip("/").split("/")[-1]

    def _replace_init_url(self, parent, url: str):
        if parent.initialization:
            parent.initialization = url

    def _replace_media_url(self, parent, url: str):
        if parent.media:
            parent.media = url


if __name__ == "__main__":

    import requests

    urls = [
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/a2d-tv.mpd",
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/ad-insertion-testcase1.mpd",
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/ad-insertion-testcase6-av1.mpd",
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/ad-insertion-testcase6-av2.mpd",
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/ad-insertion-testcase6-av5.mpd",
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/admanager.xml",
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/avod-mediatailor.mpd",
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/aws.xml",
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/dash-testcases-5b-1-thomson.mpd",
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/dashif-live-atoinf.mpd",
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/dashif-low-latency.mpd",
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/dolby-ac4.xml",
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/example_G22.mpd",
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/f64-inf.mpd",
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/jurassic-compact-5975.mpd",
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/multiple_supplementals.mpd",
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/orange.xml",
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/patch-location.mpd",
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/st-sl.mpd",
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/telenet-mid-ad-rolls.mpd",
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/telestream-binary.xml",
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/telestream-elements.xml",
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/vod-aip-unif-streaming.mpd",
        "https://github.com/avishaycohen/mpd-parser/raw/refs/heads/main/manifests/aws-media-tailor-vod-personalized-response-manifest.mpd",
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
        "https://a38avoddashs3ww-a.akamaihd.net/ondemand/iad_2/8e91/f2f2/ec5a/430f-bd7a-0779f4a0189d/685cda75-609c-41c1-86bb-688f4cdb5521_corrected.mpd",
        "http://playready.directtaps.net/smoothstreaming/SSWSS720H264/SuperSpeedway_720.ism/Manifest",
        "https://azclwds01.akamaized.net/4e8f6858-5d05-4e28-83ab-48c7a2b259e1/XVuosg_tab_hd.ism/Manifest",
        "https://media.axprod.net/TestVectors/v7-Clear/Manifest_1080p.mpd",
        "https://cmafref.akamaized.net/cmaf/live-ull/2006350/akambr/out.mpd",
        "https://bitmovin-a.akamaihd.net/content/art-of-motion_drm/mpds/11331.mpd",
    ]

    breaking_urls = [
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/mediapackage.xml",  # lxml.etree.XMLSyntaxError: Namespace prefix scte35 on SpliceInfoSection is not defined, line 30, column 89
        "https://github.com/emarsden/dash-mpd-rs/raw/refs/heads/main/tests/fixtures/incomplete.mpd",  # Breaks because incomplete
    ]

    x = 0
    y = []

    for url in breaking_urls:

        mpdRequest = DashManifestModificationRequest()
        response = DashManifestModificationResponse()
        mm = ManifestModifier()

        mpdContent = requests.get(url).text

        mpdRequest.content = mpdContent
        mpdRequest.url = url

        mm.get_modified_mpd(mpdRequest, response)

        if response.error != "0":
            print(response.error + "\n")

            with open(f"{x}.txt", "wt") as f:
                f.write(response.value)

            y.append(url)

        x += 1
