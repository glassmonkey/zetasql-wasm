# Multi-stage build for ZetaSQL WASM compilation
FROM --platform=linux/amd64 ubuntu:22.04 AS builder

# Build arguments for user/group IDs to match host user
ARG USER_ID=1000
ARG GROUP_ID=1000

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
    build-essential \
    autoconf \
    automake \
    libtool \
    m4 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create builder user and group with host user's UID/GID
# If the group already exists, use it; otherwise create it
RUN (groupadd -g ${GROUP_ID} builder 2>/dev/null || true) && \
    useradd -u ${USER_ID} -g ${GROUP_ID} -m -s /bin/bash builder

# Copy Bazel version file
COPY assets/.bazelversion /tmp/.bazelversion

# Install Bazel
RUN BAZEL_VERSION=$(cat /tmp/.bazelversion) \
    && echo "Installing Bazel ${BAZEL_VERSION}..." \
    && wget "https://github.com/bazelbuild/bazel/releases/download/${BAZEL_VERSION}/bazel-${BAZEL_VERSION}-linux-x86_64" \
    && chmod +x "bazel-${BAZEL_VERSION}-linux-x86_64" \
    && mv "bazel-${BAZEL_VERSION}-linux-x86_64" /usr/local/bin/bazel \
    && bazel --version

# Switch to builder user
USER builder

# Set working directory to builder's home
WORKDIR /home/builder/workspace

# Default command to build ZetaSQL WASM
CMD ["/bin/bash", "-c", "\
    echo 'Building ZetaSQL WASM with Bazel + Emscripten (Standalone WASM mode)...' && \
    bazel build //:zetasql && \
    echo 'Build complete!' && \
    BAZEL_BIN=$(bazel info bazel-bin) && \
    mkdir -p /home/builder/output && \
    cp \"$BAZEL_BIN/zetasql/zetasql_bridge.wasm\" /home/builder/output/zetasql.wasm && \
    echo 'Generated WASI-compatible standalone WASM' && \
    ls -lh /home/builder/output/"]
