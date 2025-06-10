from pydantic import BaseModel


class Response(BaseModel):
    error: str = "0"
    value: str | list | dict = ""


class PsshRequest(BaseModel):
    mpdUrl: str = ""
    mpdHeaders: dict = {}
    segmentsHeaders: dict = {}


class DecryptRequest(BaseModel):
    pssh: str = ""
    licenseUrl: str = ""
    licenseHeaders: dict[str, str] = {}


class MpdRequest(BaseModel):
    mpdUrl: str = ""
    mpdHeaders: dict = {}
