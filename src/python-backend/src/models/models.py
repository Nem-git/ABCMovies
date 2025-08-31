from pydantic import BaseModel


class BasicResponse(BaseModel):
    error: str = "0"


class WidevinePsshRequest(BaseModel):
    url: str = ""
    headers: dict[str, str] = {}
    segheaders: dict[str, str] = {}


class WidevinePsshResponse(BasicResponse):
    pssh: str = ""


class WidevineKeysRequest(BaseModel):
    pssh: str = ""
    url: str = ""
    headers: dict[str, str] = {}


class WidevineKeysResponse(BasicResponse):
    keys: list[str] = []


class DashManifestModificationRequest(BaseModel):
    url: str = ""
    content: str = ""


class DashManifestModificationResponse(BasicResponse):
    content: str = ""
