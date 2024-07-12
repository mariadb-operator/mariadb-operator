FROM alpine:3.20.1

RUN apk add -U --no-cache \
  python3 \
  python3-dev \
  py3-pip \
  musl-dev \
  git \
  openssh \
  git-fast-import \
  bash \
  gcc \
  diffutils

ENV PATH=$PATH:/.venv/bin

COPY requirements.txt /

RUN python3 -m venv .venv && \
  source .venv/bin/activate && \
  pip3 install --upgrade pip && \
  pip3 install -r /requirements.txt

# Disable the top-level directory owner check
# https://github.com/git/git/commit/8959555cee7ec045958f9b6dd62e541affb7e7d9
RUN git config --system --add safe.directory '*'

WORKDIR /docs

EXPOSE 8000