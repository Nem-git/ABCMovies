import uvicorn
from fastapi import FastAPI

from src.models.models import *

from src.app.pssh import Pssh
from src.app.decryption import Widevine
from src.app.mpd import ManifestModifier

app = FastAPI()


@app.post("/pssh")
def create_pssh(request: WidevinePsshRequest) -> WidevinePsshResponse:
    extractor = Pssh()
    response = WidevinePsshResponse()
    extractor.get_pssh(request, response)
    return response


@app.post("/decryptionKeys")
def create_decryption_keys(request: WidevineKeysRequest) -> WidevineKeysResponse:
    retriever = Widevine()
    response = WidevineKeysResponse()
    retriever.get_keys(request, response)
    return response


@app.post("/manifest")
def create_modified_manifest(
    request: DashManifestModificationRequest,
) -> DashManifestModificationResponse:
    modifier = ManifestModifier()
    response = DashManifestModificationResponse()
    modifier.get_modified_mpd(request, response)

    return response
