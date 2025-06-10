import urllib
import base64
import requests
import re
from mpd_parser.parser import Parser
from mpd_parser.models.composite_tags import MPD
import os

from app.models import MpdRequest
from app.models import Response


# TODO: Look for baseurls, and use them for the urlencoding
# TODO: Look at base_urls, segment_bases, segment_lists and segment_templates for URLS to take into account
# TODO: Remove/replace base_urls


class ManifestModifier:

    manifest_url: str
    mpd: MPD

    def get_mpd(self, request: MpdRequest, response: Response) -> str:

        try:
            self.manifest_url = request.mpdUrl

            self.mpd = Parser.from_string(
                self._get_mpd_content(request.mpdUrl, request.mpdHeaders)
            )

            self._parse_periods(self.mpd)

        except Exception as e:
            response.error = e

        response.value = Parser.to_string(self.mpd)

    def _get_mpd_content(self, url: str, headers: dict) -> str:
        response = requests.get(url, headers=headers)
        response.raise_for_status()
        return response.text

    def _parse_periods(self, parent):
        for period in parent.periods:
            self._parse_adaptation_sets(period)

    def _parse_adaptation_sets(self, parent):
        for adaptation_set in parent.adaptation_sets:
            self._parse_segment_templates(adaptation_set)
            self._parse_representations(adaptation_set)

    def _parse_representations(self, parent):
        for representation in parent.representations:
            self._parse_segment_templates(representation)

    def _parse_segment_templates(self, parent):
        for segment_template in parent.segment_templates:

            init_base_url = self._base_64_encode(
                self._find_base_url(parent, segment_template.initialization)
            )
            media_base_url = self._base_64_encode(
                self._find_base_url(parent, segment_template.media)
            )

            init_path = self._clean_segment_url(segment_template.initialization)
            media_path = self._clean_segment_url(segment_template.media)

            init_url = os.path.join("init", init_base_url, init_path)
            media_url = os.path.join("media", media_base_url, media_path)

            self._replace_init_url(segment_template, init_url)
            self._replace_media_url(segment_template, media_url)

    def _base_64_encode(self, url: str) -> str:
        return base64.b64encode(url.encode("utf-8")).decode("utf-8")

    def _urlencoded(self, url: str) -> str:
        return urllib.parse.quote_plus(url)

    def _base_url_from_url(self, url) -> str:
        return os.path.dirname(url) + "/"

    # I only use urllib.parse.urljoin here and not os.path.join, because it takes ../ paths and
    # is meant for full urls, not relatives like in the other cases
    def _find_base_url(self, parent, segment_url):
        base_url = ""

        # If there is a baseurl, add it firt to the base_url
        if len(parent.base_urls) > 0:
            base_url = urllib.parse.urljoin(base_url, parent.base_urls[0])
        else:
            base_url = urllib.parse.urljoin(
                base_url, self._base_url_from_url(self.manifest_url)
            )

        re_match = re.search(r"^[\./]+", segment_url)

        if re_match:
            # Only matters if there are ../../ in front of the segment url template
            base_url = urllib.parse.urljoin(base_url, re_match.group())

        return base_url

    def _clear_base_url(self, parent):
        parent.base_urls = []

    def _replace_init_url(self, parent, url):
        parent.initialization = url

    def _replace_media_url(self, parent, url):
        parent.media = url

    def _clean_segment_url(self, url: str) -> str:
        parsed_url = urllib.parse.urlsplit(url)
        clean_url = ""

        # Takes only what is after https:// and ../
        # Should always match (except for rare cases, like images)
        re_match = re.search(r"[^./]+[\w\W]*$", parsed_url.path)

        if re_match:
            clean_url = re_match.group()

        if len(parsed_url.query) > 0:
            clean_url += "?" + parsed_url.query

        # Should return a cleaned URL, from: ../file.mp4 to: file.mp4
        # or from: https://video.com/file.mp4 to file.mp4
        # (don't worry, the video.com part is saved as urlencoded base_url)
        return clean_url
