import uvicorn
from fastapi import FastAPI

from src.models.models import Response
from src.models.models import PsshRequest
from src.models.models import DecryptRequest
from src.models.models import MpdRequest

from src.app.pssh import Pssh
from src.app.decryption import Widevine
from src.app.mpd import ManifestModifier

app = FastAPI()


@app.post("/pssh")
def create_pssh(request: PsshRequest):
    extractor = Pssh()
    response = Response()
    extractor.get_pssh(request, response)
    return response


@app.post("/decryptionKeys")
def create_decryption_keys(request: DecryptRequest):
    retriever = Widevine()
    response = Response()
    retriever.get_keys(request, response)
    return response


@app.post("/manifest")
def create_modified_manifest(request: MpdRequest):
    modifier = ManifestModifier()
    response = Response()
    modifier.get_modified_mpd(request, response)
    return response
