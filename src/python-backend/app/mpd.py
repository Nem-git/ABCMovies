import urllib
import base64
import os
import requests
import re
from lxml import etree
from mpd_parser.parser import Parser
from mpd_parser.models.composite_tags import MPD
from uuid import uuid4

# Production
from app.models import MpdRequest
from app.models import Response

# Development
# from models import MpdRequest
# from models import Response


class ManifestModifier:

    mpd_url: str
    mpd_content: str

    base_url: str
    mpd_object: Parser
   

    def get_modified_mpd(self, request: MpdRequest, response: Response) -> str:
        try:
            self.mpd_url = request.mpdUrl
            self.mpd_content = request.mpdContent
            # Sets the base url from the manifest
            self.mpd_object = Parser.from_string(self.mpd_content)
            self._set_initial_base_url()

            self._parse_xml()
        except Exception as e:
            response.error = str(e)
            return

        # Apply XML modifications (cleaning up DRM, etc.)
        mpd_str = Parser.to_string(self.mpd_object)
        mpd_str = self._apply_lxml_modifications(mpd_str)

        response.value = mpd_str

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
        root = self._remove_drm_namespaces(root)

        return etree.tostring(root, encoding="unicode", pretty_print=True)

    def _remove_drm_namespaces(self, root):
        mpd_content = etree.tostring(root, encoding="unicode")

        # Remove DRM namespaces and associated elements
        mpd_content = re.sub(r'\sxmlns:mspr="urn:microsoft:playready"', "", mpd_content)
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
    
    def _remove_base_url(self, root):
        # Remove <ContentProtection> tags
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
                self._parse_segment_templates(adaptation_set, adaptation_set_base_url)

                for representation in adaptation_set.representations:
                    representation_base_url = self._extend_base_url(
                        representation, adaptation_set_base_url
                    )
                    self._parse_segment_templates(
                        representation, representation_base_url
                    )

    def _parse_segment_templates(self, parent, base_url=""):
        for segment_template in parent.segment_templates:

            init_media_id = self._get_unique_id()

            init_url = self._join_url(base_url, segment_template.initialization)
            formatted_init_path = self._get_formatted_path(init_url)

            media_url = self._join_url(base_url, segment_template.media)
            formatted_media_path = self._get_formatted_path(media_url)

            # Init Url also needs to know it's own checksum
            # Because of changing values in the url, like $RepresentationId$
            # it would be impossible to link the init with the media
            init_url = self._construct_segment_url(
                "dash/init", formatted_init_path, init_url, init_media_id
            )
            media_url = self._construct_segment_url(
                "dash/media", formatted_media_path, init_url, init_media_id
            )

            # Replace URLs in the segment template
            self._replace_init_url(segment_template, init_url)
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
        self, segment_type: str, formatted_path: str, init_url: str, init_media_id: str
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
    mpdRequest = MpdRequest()
    response = Response()
    mm = ManifestModifier()

    url = "https://raw.githubusercontent.com/emarsden/dash-mpd-cli/refs/heads/main/tests/fixtures/jurassic-compact-5975.mpd"
    mpdContent = requests.get(url).text
    
    mpdRequest.mpdContent = mpdContent
    mpdRequest.mpdUrl = url

    mm.get_modified_mpd(mpdRequest, response)

    with open("file.xml", "wt") as f:
        f.write(response.value)
