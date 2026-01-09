# Multi-stage build for ZetaSQL WASM compilation

# Stage 1: Base dependencies installation
FROM --platform=linux/amd64 ubuntu:22.04 AS base-deps

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

# Stage 3: Builder environment with user setup
FROM bazel-installer AS builder

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
