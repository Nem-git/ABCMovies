from fastapi import FastAPI

from app.models import PsshRequest
from app.models import PsshResponse
from app.models import DecryptRequest
from app.models import DecryptResponse

from app.pssh import Pssh
from app.decryption import Widevine

app = FastAPI()


@app.post("/pssh")
def create_pssh(request: PsshRequest):
    extractor = Pssh()
    response = PsshResponse()
    response = extractor.get_pssh(request)
    return response

@app.post("/decrypt")
def create_decryption_keys(request: DecryptRequest):
    retriever = Widevine()
    decrypt_response = DecryptResponse()
    response = retriever.get_keys(request)
    return response
