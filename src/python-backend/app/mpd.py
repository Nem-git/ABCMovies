import urllib
import base64
import requests
import re
from mpd_parser.parser import Parser
from mpd_parser.models.composite_tags import MPD
import os
from lxml import etree

# Removed just for debugging
from app.models import MpdRequest
from app.models import Response

# from models import MpdRequest
# from models import Response


# TODO: Look for baseurls, and use them for the urlencoding
# TODO: Look at base_urls, segment_bases, segment_lists and segment_templates for URLS to take into account
# TODO: Remove/replace base_urls


class ManifestModifier:

    manifest_url: str
    mpd: MPD

    def get_modified_mpd(self, request: MpdRequest, response: Response) -> str:

        try:
            self.manifest_url = request.mpdUrl

            self.mpd = Parser.from_string(
                self._get_mpd_content(request.mpdUrl, request.mpdHeaders)
            )

            self._parse_periods(self.mpd)

        except Exception as e:
            response.error = e

        str_mpd: str = Parser.to_string(self.mpd)
        str_mpd: str = self._remove_content_protection_lxml(str_mpd)
        str_mpd: str = self._remove_drm_namespace(str_mpd)
        mpd: MPD = Parser.from_string(str_mpd)
        response.value = Parser.to_string(mpd)
    
    def _remove_drm_namespace(self, mpd_xml_str: str):

        drm_ns_prefixes = {'mspr', 'cenc', 'widevine'}
    
        parser = etree.XMLParser(remove_blank_text=True)
        root = etree.fromstring(mpd_xml_str.encode('utf-8'), parser)
    
        # Build a new nsmap without DRM namespaces
        new_nsmap = {k: v for k, v in root.nsmap.items() if k not in drm_ns_prefixes}
    
        # Create a new root element with the same tag, text, tail, attributes, but filtered nsmap
        new_root = etree.Element(root.tag, nsmap=new_nsmap)
    
        # Copy attributes (excluding xmlns declarations, which are handled by nsmap)
        for attr_key, attr_val in root.attrib.items():
            new_root.set(attr_key, attr_val)
    
        # Copy children
        for child in root:
            new_root.append(child)
    
        # Copy text and tail
        new_root.text = root.text
        new_root.tail = root.tail
    
        # Serialize back to string with pretty print
        return etree.tostring(new_root, pretty_print=True, encoding='unicode')
    
    def _remove_content_protection_lxml(self, mpd_xml_str: str) -> str:
        parser = etree.XMLParser(ns_clean=True, recover=True)
        root = etree.fromstring(mpd_xml_str.encode('utf-8'), parser=parser)
    
        cps = root.xpath('.//mpd:ContentProtection', namespaces={'mpd': 'urn:mpeg:dash:schema:mpd:2011'})

        for cp in cps:
            parent = cp.getparent()
            if parent is not None:
                parent.remove(cp)
        
        return etree.tostring(root, encoding='unicode', pretty_print=True)

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

            init_base_url = self._find_base_url(parent, segment_template.initialization)
            media_base_url = self._find_base_url(parent, segment_template.media)

            init_path = self._clean_segment_url(segment_template.initialization)
            media_path = self._clean_segment_url(segment_template.media)

            init_url = os.path.join(
                "init", self._base_64_encode(init_base_url), init_path
            )
            # Added the init URL, to be able to identify and select the right init content in the DB for merging
            b64_init_url = self._base_64_encode(
                urllib.parse.urljoin(init_base_url, init_path)
            )

            media_url = os.path.join(
                "media", b64_init_url, self._base_64_encode(media_base_url), media_path
            )

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


# if __name__ == "__main__":
#     mpd_request = MpdRequest()
#     mpd_request.mpdUrl = "https://cbcrcott-aws-toutv.akamaized.net/out/v1/e71b9f2ee8684145982294c54e2311ed/97c7a58d11d84ea78801a32f293d0a21/27f2eb30c8fb43f99ba46fee14ce2d37/index-multi-drm.mpd?pckgrp=bd19b98e3f6f49156464835f3aa1e8bb&ewid=83314&filter=3000&EIA608ClosedCaptions=true&lang=fr"
#     response = Response()

#     manifest_modifier = ManifestModifier()
#     manifest_modifier.get_mpd(mpd_request, response)

#     print(response.value.count("Protection"))

#     with open("file.xml", "wt") as f:
#         f.write(response.value)
