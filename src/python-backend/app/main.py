import uvicorn
from fastapi import FastAPI

from models import Response
from models import PsshRequest
from models import DecryptRequest
from models import MpdRequest

from pssh import Pssh
from decryption import Widevine
from mpd import ManifestModifier

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


if __name__ == "__main__":
    uvicorn.run("main:app", port=8000, log_level="info")
