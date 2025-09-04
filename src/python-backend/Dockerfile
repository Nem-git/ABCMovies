
############### PYTHON BACKEND BASE #############

FROM python:3-alpine AS python-backend-base

WORKDIR /srv/www

COPY ./requirements.txt ./requirements.txt

RUN pip install --no-cache-dir --upgrade -r requirements.txt

EXPOSE 8000

################ PYTHON BACKEND DEV ################

FROM python-backend-base AS python-backend-dev

CMD ["uvicorn", "src.main:app", "--host", "0.0.0.0", "--port", "8000"]

############ PYTHON BACKEND PROD ####################

FROM python-backend-base AS python-backend-prod

COPY src ./src

CMD ["uvicorn", "src.main:app", "--host", "0.0.0.0", "--port", "8000"]