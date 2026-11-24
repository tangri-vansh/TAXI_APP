FROM ubuntu:24.04 AS builder

ARG VALHALLA_VERSION=3.9.1
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends git sudo ca-certificates && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /src/valhalla
RUN git clone --depth 1 --branch "${VALHALLA_VERSION}" https://github.com/valhalla/valhalla.git .
COPY backend/internal/routing/valhalla-safe-mode.patch /tmp/valhalla-safe-mode.patch
RUN git apply /tmp/valhalla-safe-mode.patch && bash scripts/install-linux-deps.sh
RUN git submodule update --init --recursive

RUN cmake -B build \
      -DCMAKE_BUILD_TYPE=Release \
      -DCMAKE_C_COMPILER=gcc \
      -DENABLE_PYTHON_BINDINGS=Off \
      -DENABLE_TESTS=Off \
      -DENABLE_SINGLE_FILES_WERROR=Off && \
    cmake --build build --target install --parallel 2

FROM ghcr.io/valhalla/valhalla-scripted:3.9.1
COPY --from=builder /usr/local /usr/local
