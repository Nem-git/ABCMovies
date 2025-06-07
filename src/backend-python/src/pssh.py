import base64
from base64 import b64encode
import requests
import re
import xml.etree.ElementTree as ET
import os
import yt_dlp

from models import PsshRequest
from models import PsshResponse


class Pssh:
    WIDEVINE_SYSTEM_ID: str = 'EDEF8BA9-79D6-4ACE-A3C8-27DCD51D21ED'

    def get_pssh(
        self,
        request: PsshRequest
    ) -> str | None:
        """
        Extract or generate the PSSH from the MPD manifest or init segment.
        """

        response = PsshResponse()

        try:
            mpd_content: bytes = self._fetch_content(request.mpd_url, request.mpd_headers)
            response.pssh: str = self._extract_or_generate_pssh(request.mpd_url, mpd_content, request.mpd_headers, request.segments_headers)
            
            if not response.pssh:
                response.error = "Was not able to find the PSSH"
            
        except Error as e:
            response.error = e
        
        return response

    def _fetch_content(self, url: str, headers: dict[str, str]) -> bytes:
        response = requests.get(url, headers=headers)
        response.raise_for_status()
        return response.content

    def _find_default_kid_with_regex(self, mpd_content: bytes) -> str | None:
        match = re.search(r'cenc:default_KID="([A-F0-9-]+)"', mpd_content.decode(errors='ignore'))
        if match:
            return match.group(1)
        return None

    def _extract_or_generate_pssh(
        self,
        mpd_url: str,
        mpd_content: bytes,
        mpd_headers: dict[str, str],
        segments_headers: dict[str, str]
    ) -> str:

        tree = ET.ElementTree(ET.fromstring(mpd_content))
        root = tree.getroot()

        namespaces = {
            'cenc': 'urn:mpeg:cenc:2013',
            '': 'urn:mpeg:dash:schema:mpd:2011'
        }

        default_kid: str | None = None
        for elem in root.findall('.//ContentProtection', namespaces):
            scheme_id_uri = elem.attrib.get('schemeIdUri', '').upper()
            if scheme_id_uri == 'URN:MPEG:DASH:MP4PROTECTION:2011':
                default_kid = elem.attrib.get('cenc:default_KID')
                if default_kid:
                    print(f"Found default_KID using XML parsing: {default_kid}")
                    break

        if not default_kid:
            default_kid = self._find_default_kid_with_regex(mpd_content)
            if default_kid:
                print(f"Found default_KID using regex: {default_kid}")

        pssh: str | None = None
        for elem in root.findall('.//ContentProtection', namespaces):
            scheme_id_uri = elem.attrib.get('schemeIdUri', '').upper()
            if scheme_id_uri == f'URN:UUID:{self.WIDEVINE_SYSTEM_ID}':
                pssh_elem = elem.find('cenc:pssh', namespaces)
                if pssh_elem is not None and pssh_elem.text:
                    pssh = pssh_elem.text.strip()
                    print(f"Found pssh element: {pssh}")
                    break

        if pssh is not None:
            return pssh
        elif default_kid is not None:
            default_kid_clean = default_kid.replace('-', '')
            s = f'000000387073736800000000edef8ba979d64acea3c827dcd51d21ed000000181210{default_kid_clean}48e3dc959b06'
            return b64encode(bytes.fromhex(s)).decode()
        else:
            return self._get_pssh_from_mpd(mpd_url, mpd_headers, segments_headers)

    def _find_wv_pssh_offsets(self, raw: bytes) -> list[bytes]:
        offsets: list[bytes] = []
        offset = 0
        while True:
            offset = raw.find(b'pssh', offset)
            if offset == -1:
                break
            size = int.from_bytes(raw[offset - 4:offset], byteorder='big')
            pssh_offset = offset - 4
            offsets.append(raw[pssh_offset:pssh_offset + size])
            offset += size
        return offsets

    def _to_pssh(self, content: bytes) -> list[str]:
        wv_offsets = self._find_wv_pssh_offsets(content)
        return [base64.b64encode(wv_offset).decode() for wv_offset in wv_offsets]

    def _get_pssh_from_mpd(
        self,
        mpd_url: str,
        mpd_headers: dict[str, str],
        segments_headers: dict[str, str]
    ) -> str:

        # TODO: Add the mpd_headers to the yt-dlp request

        ydl_opts = {
            'format': 'bestvideo[ext=mp4]/bestaudio[ext=m4a]/best',
            'allow_unplayable_formats': True,
            'no_warnings': True,
            'quiet': True,
            "simulate": True,
            'test': True,
        }

        with yt_dlp.YoutubeDL(ydl_opts) as ydl:
            info = ydl.extract_info(mpd_url, download=False)

        init_url: str | None = None
        for f in info.get("formats", []):
            if f.get("vcodec") != "none" or f.get("acodec") != "none":
                base_url = f.get("fragment_base_url", "")
                segment_path: str | None = None

                for segment in f.get("fragments", []):
                    if segment.get("path"):
                        segment_path = segment["path"]
                        break

                if base_url and segment_path:
                    init_url = base_url + segment_path
                    break

        if not init_url:
            print("Init segment URL not found.")
            return ""

        init_content = self._fetch_content(init_url, segments_headers)

        pssh_list = self._to_pssh(init_content)

        pssh = ""

        for target_pssh in pssh_list:
            if 20 < len(target_pssh) < 220:
                pssh = target_pssh

        print(f"PSSH:\n{pssh}\n")

        return pssh