FROM python:3-alpine AS builder

WORKDIR /app

ENV BENTO4_VERSION=1-6-0-641

ENV BENTO4_INSTALL_DIR=/opt/bento4
ENV BENTO_FILENAME=Bento4-SDK-${BENTO4_VERSION}.x86_64-unknown-linux

RUN mkdir -p ${BENTO4_INSTALL_DIR}

RUN apk --no-cache add wget unzip

RUN cd /tmp/
RUN wget -O ${BENTO_FILENAME}.zip https://www.bok.net/Bento4/binaries/${BENTO_FILENAME}.zip
RUN unzip ${BENTO_FILENAME}.zip
RUN rm ${BENTO_FILENAME}.zip
RUN cp -r ${BENTO_FILENAME}/* ${BENTO4_INSTALL_DIR}
RUN rm -r ${BENTO_FILENAME}/

############################################################################################################

FROM python:3-alpine

WORKDIR /srv/www

ENV BENTO4_INSTALL_DIR=/opt/bento4
ENV PATH=/opt/bento4/bin:/home/appuser/.local/bin:${PATH}

RUN addgroup -S appgroup
RUN adduser -S appuser -G appgroup

RUN mkdir -p ${BENTO4_INSTALL_DIR}

COPY --from=builder ${BENTO4_INSTALL_DIR} ${BENTO4_INSTALL_DIR}

COPY ./requirements.txt ./requirements.txt

RUN pip install --no-cache-dir --upgrade -r requirements.txt

COPY src ./src

EXPOSE 8000

USER appuser

CMD ["uvicorn", "src.main:app", "--host", "0.0.0.0", "--port", "8000"]
# Add --reload when in Dev