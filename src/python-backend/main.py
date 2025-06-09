from fastapi import FastAPI

from app.models import Response
from app.models import PsshRequest
from app.models import DecryptRequest

from app.pssh import Pssh
from app.decryption import Widevine

app = FastAPI()


@app.post("/pssh")
def create_pssh(request: PsshRequest):
    extractor = Pssh()
    response = Response()
    extractor.get_pssh(request, response)
    return response


@app.post("/decrypt")
def create_decryption_keys(request: DecryptRequest):
    retriever = Widevine()
    response = Response()
    retriever.get_keys(request, response)
    return response
