FROM golang:1.26.6

RUN apt-get update \
 && apt-get install -y --no-install-recommends curl git make ca-certificates \
 && curl -fsSL https://mise.jdx.dev/install.sh | sh \
 && rm -rf /var/lib/apt/lists/*

ENV PATH="/root/.local/bin:/root/.local/share/mise/shims:${PATH}"

WORKDIR /workspace

COPY .tool-versions ./
RUN mise install

COPY . .

RUN make deps && make proto && make build

CMD ["make", "run"]
