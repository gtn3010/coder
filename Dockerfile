ARG BUILDERIMAGE="golang:1.24.10"
ARG BASEIMAGE="alpine:3.21.0"

FROM $BUILDERIMAGE as builder

WORKDIR /app

ENV NODE_VERSION="v22.19.0"
ENV PNPM_VERSION="v10.21.0"
ENV CODER_FORCE_VERSION="2.27.6"
ENV NODE_OPTIONS="--max-old-space-size=7000"

RUN apt update && apt install -y unzip postgresql-client && \
    CGO_ENABLED=1 go install github.com/coder/sqlc/cmd/sqlc@aab4e865a51df0c43e1839f81a9d349b41d14f05 && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.30 && \
    go install go.uber.org/mock/mockgen@latest && \
    wget https://github.com/pnpm/pnpm/releases/download/${PNPM_VERSION}/pnpm-linux-x64 && \
    mv pnpm-linux-x64 /usr/local/bin/pnpm && \
    chmod +x /usr/local/bin/pnpm && \
    wget https://nodejs.org/download/release/${NODE_VERSION}/node-${NODE_VERSION}-linux-x64.tar.gz && \
    tar -xf node-${NODE_VERSION}-linux-x64.tar.gz && \
    mv node-${NODE_VERSION}-linux-x64 /node && \
    git clone https://github.com/facebook/zstd && cd zstd && make install && \
    mkdir /protoc && cd /protoc && wget https://github.com/protocolbuffers/protobuf/releases/download/v33.0/protoc-33.0-linux-x86_64.zip && unzip protoc-33.0-linux-x86_64.zip

COPY . .

RUN PATH="${PATH}:/protoc/bin" PATH=$PATH:/node/bin ./scripts/build_go.sh --os linux --arch amd64 --version $CODER_FORCE_VERSION --output build/

FROM $BASEIMAGE as base

ENV TERRAFORM_VERSION="1.12.2"

RUN apk add --no-cache \
                curl \
                wget \
                bash \
                git \
                openssl \
                busybox-extras \
                openssh-client && \
        addgroup \
                -g 1000 \
                coder && \
        adduser \
                -D \
                -s /bin/bash \
                -h /home/coder \
                -u 1000 \
                -G coder \
                coder
# Terraform was disabled in the edge repo due to a build issue.
# https://gitlab.alpinelinux.org/alpine/aports/-/commit/f3e263d94cfac02d594bef83790c280e045eba35
# Using wget for now. Note that busybox unzip doesn't support streaming.
RUN ARCH="$(arch)"; if [ "${ARCH}" == "x86_64" ]; then ARCH="amd64"; elif [ "${ARCH}" == "aarch64" ]; then ARCH="arm64"; fi; wget -O /tmp/terraform.zip "https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_${ARCH}.zip" && \
                busybox unzip /tmp/terraform.zip -d /usr/local/bin && \
                rm -f /tmp/terraform.zip && \
                chmod +x /usr/local/bin/terraform && \
                terraform --version

USER 1000:1000
ENV HOME=/home/coder
ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/opt
WORKDIR /home/coder

# COPY --chown=1000:1000 --chmod=755 coder /opt/coder
COPY --from=builder /app/build/coder /opt/coder

ENTRYPOINT [ "/opt/coder", "server" ]