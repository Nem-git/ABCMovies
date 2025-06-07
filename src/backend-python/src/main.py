from fastapi import FastAPI

from models import PsshRequest
from models import PsshResponse
from models import DecryptRequest
from models import DecryptResponse

from pssh import Pssh
from decryption import Widevine

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
