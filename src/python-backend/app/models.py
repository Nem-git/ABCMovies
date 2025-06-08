from pydantic import BaseModel


class PsshRequest(BaseModel):
    mpdUrl: str = ""
    mpdHeaders: dict = {}
    segmentsHeaders: dict = {}


class PsshResponse(BaseModel):
    error: str = "0"
    pssh: str = ""


class DecryptRequest(BaseModel):
    pssh: str = ""
    licenseUrl: str = ""
    licenseHeaders: dict[str, str] = {}


class DecryptResponse(BaseModel):
    error: str = "0"
    decryptionKeys: list = ""
