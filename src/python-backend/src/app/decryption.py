from pywidevine.cdm import Cdm
from pywidevine.device import Device
from pywidevine.pssh import PSSH
import base64
import os
import requests

from src.models.models import DecryptRequest
from src.models.models import Response
from src.config.constants import WVD_PATH


class Widevine:

    def __init__(self):
        self.device = Device.load(WVD_PATH)
        self.cdm = Cdm.from_device(self.device)

    def get_keys(self, request: DecryptRequest, response: Response) -> list[str]:

        try:
            """
            Given a PSSH and license URL, request license and extract keys.
            """

            session_id = self.cdm.open()
            payload = self.cdm.get_license_challenge(session_id, PSSH(request.pssh))

            license_response = requests.post(
                url=request.licenseUrl, data=payload, headers=request.licenseHeaders
            )
            license_response.raise_for_status()

            license_content = license_response.content

            if isinstance(license_content, str):
                license_content = base64.b64decode(license_content)

            self.cdm.parse_license(session_id, license_content)

            keys: list[str] = []
            for key in self.cdm.get_keys(session_id):
                if key.type == "CONTENT":
                    keys.append(f"{key.kid.hex}:{key.key.hex()}")

            self.cdm.close(session_id)

            if len(keys) == 0:
                response.error = "No keys found"

            response.value = keys
        except Exception as e:
            response.error = e
