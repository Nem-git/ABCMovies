from pydantic import BaseModel


class PsshRequest(BaseModel):
    mpd_url: str = ""
    mpd_headers: dict = {}
    segments_headers: dict = {}


class PsshResponse(BaseModel):
    error: str = "0"
    pssh: str = ""


class DecryptRequest(BaseModel):
    pssh: str = ""
    license_url: str = ""
    license_headers: dict[str, str] = {}


class DecryptResponse(BaseModel):
    error: str = "0"
    decryption_keys: list = ""
