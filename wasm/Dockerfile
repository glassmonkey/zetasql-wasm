# Multi-stage build for ZetaSQL WASM compilation

# Stage 1: Base dependencies installation
FROM --platform=linux/amd64 ubuntu:24.04 AS base-deps

# Install required tools
# Bazel will automatically download Emscripten toolchain
# autoconf/automake/libtool are required for ICU build
RUN apt-get update -y && \
    apt-get install -y --no-install-recommends \
    git \
    python3 \
    python3-pip \
    wget \
    curl \
    unzip \
    build-essential \
    autoconf \
    automake \
    libtool \
    m4 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Install protoc 33.5 manually to support Protocol Buffers Edition 2023
# Edition 2023 requires protoc 27.0 or later (Ubuntu 24.04 APT provides only 3.21.12)
RUN PROTOC_VERSION=33.5 && \
    echo "Installing protoc ${PROTOC_VERSION}..." && \
    wget "https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip" && \
    unzip "protoc-${PROTOC_VERSION}-linux-x86_64.zip" -d /tmp/protoc && \
    mv /tmp/protoc/bin/protoc /usr/local/bin/protoc && \
    mv /tmp/protoc/include/* /usr/local/include/ && \
    chmod +x /usr/local/bin/protoc && \
    rm -rf /tmp/protoc "protoc-${PROTOC_VERSION}-linux-x86_64.zip" && \
    protoc --version

# Copy go.mod to determine Go version
COPY go.mod /tmp/go.mod

# Install Go based on version specified in go.mod
# Extract major.minor version (e.g., "1.23") and find the latest patch version
RUN GO_MAJOR_MINOR=$(grep "^go " /tmp/go.mod | awk '{print $2}') && \
    echo "Looking for latest Go ${GO_MAJOR_MINOR}.x version..." && \
    GO_VERSION=$(curl -s https://go.dev/dl/?mode=json | \
        python3 -c "import sys, json; versions = json.load(sys.stdin); print(next((v['version'][2:] for v in versions if v['version'].startswith('go${GO_MAJOR_MINOR}.')), '${GO_MAJOR_MINOR}.0'))" 2>/dev/null || echo "${GO_MAJOR_MINOR}.0") && \
    echo "Installing Go ${GO_VERSION}..." && \
    wget -O go.tar.gz "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" && \
    tar -C /usr/local -xzf go.tar.gz && \
    rm go.tar.gz && \
    /usr/local/go/bin/go version

ENV PATH="/usr/local/go/bin:/root/go/bin:${PATH}"

# Install protoc-gen-go to /usr/local/bin (accessible by all users)
RUN GOBIN=/usr/local/bin go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    protoc-gen-go --version

# Stage 2: Bazel installation
FROM base-deps AS bazel-installer

# Copy Bazel version file
COPY assets/.bazelversion /tmp/.bazelversion

# Install Bazel
RUN BAZEL_VERSION=$(cat /tmp/.bazelversion) \
    && echo "Installing Bazel ${BAZEL_VERSION}..." \
    && wget "https://github.com/bazelbuild/bazel/releases/download/${BAZEL_VERSION}/bazel-${BAZEL_VERSION}-linux-x86_64" \
    && chmod +x "bazel-${BAZEL_VERSION}-linux-x86_64" \
    && mv "bazel-${BAZEL_VERSION}-linux-x86_64" /usr/local/bin/bazel \
    && bazel --version

# Stage 3: Build code generation tools
FROM bazel-installer AS tool-builder

# Copy Go module and tools
COPY go.mod go.sum /tmp/wasm/
COPY tools /tmp/wasm/tools

# Build extract-protos and protogen tools
RUN cd /tmp/wasm && \
    go mod download && \
    echo "Building extract-protos..." && \
    go build -o /usr/local/bin/extract-protos ./tools/extract-protos && \
    echo "Building protogen..." && \
    go build -o /usr/local/bin/protogen ./tools/protogen && \
    chmod +x /usr/local/bin/extract-protos /usr/local/bin/protogen && \
    echo "Testing tools..." && \
    extract-protos --help && \
    protogen --help

# Stage 4: Builder environment with user setup
FROM tool-builder AS builder

# Build arguments for user/group IDs to match host user
ARG USER_ID=1000
ARG GROUP_ID=1000

# Create builder user and group with host user's UID/GID
# If the group already exists, use it; otherwise create it
RUN (groupadd -g ${GROUP_ID} builder 2>/dev/null || true) && \
    useradd -u ${USER_ID} -g ${GROUP_ID} -m -s /bin/bash builder

# Switch to builder user
USER builder

# Set working directory to builder's home
WORKDIR /home/builder/workspace

# Copy Bazel configuration and source files
COPY --chown=builder:builder assets/.bazelrc /home/builder/workspace/.bazelrc
COPY --chown=builder:builder assets/.bazelversion /home/builder/workspace/.bazelversion
COPY --chown=builder:builder assets/MODULE.bazel /home/builder/workspace/MODULE.bazel
COPY --chown=builder:builder assets/BUILD.bazel /home/builder/workspace/BUILD.bazel
COPY --chown=builder:builder assets/bridge.cc /home/builder/workspace/bridge.cc
COPY --chown=builder:builder assets/patches/ /home/builder/workspace/patches/

# Copy build script
COPY --chown=builder:builder assets/docker-build.sh /home/builder/scripts/build.sh
RUN chmod +x /home/builder/scripts/build.sh

# Default command to run the build
CMD ["/home/builder/scripts/build.sh"]
