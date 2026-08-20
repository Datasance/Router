# registry.access.redhat.com/ubi9/ubi-minimal:latest — pin manifest list digest
FROM registry.access.redhat.com/ubi9/ubi-minimal@sha256:8eb2830d0936237fc13a1f2f7e45aecf90d69043380ad167fad0343632937f41 AS builder

# upgrade first to avoid fixable vulnerabilities
# do this in builder as well as in buildee, so builder does not have different pkg versions from buildee image
RUN microdnf -y upgrade --refresh --best --nodocs --noplugins --setopt=install_weak_deps=0 --setopt=keepcache=0 \
 && microdnf clean all -y

RUN microdnf -y --setopt=install_weak_deps=0 --setopt=tsflags=nodocs install \
    rpm-build \
    gcc gcc-c++ make cmake pkgconfig \
    cyrus-sasl-devel openssl-devel libuuid-devel \
    python3-devel python3-pip python3-wheel \
    libnghttp2-devel \
    wget tar patch findutils git \
    libtool \
 && microdnf clean all -y

WORKDIR /build
# Clone skupper-router 3.5.2 so repo contents are in /build (not /build/skupper-router)
RUN git clone --depth 1 --branch 3.5.2 https://github.com/skupperproject/skupper-router.git .
ENV PROTON_VERSION=e5d5c2badb964684bf41ba509a110bf06a24712a
ENV PROTON_SOURCE_URL=${PROTON_SOURCE_URL:-https://github.com/apache/qpid-proton/archive/${PROTON_VERSION}.tar.gz}
ENV LWS_VERSION=v4.3.3
ENV LIBUNWIND_VERSION=v1.8.1
ENV LWS_SOURCE_URL=${LWS_SOURCE_URL:-https://github.com/warmcat/libwebsockets/archive/refs/tags/${LWS_VERSION}.tar.gz}
ENV LIBUNWIND_SOURCE_URL=${LIBUNWIND_SOURCE_URL:-https://github.com/libunwind/libunwind/archive/refs/tags/${LIBUNWIND_VERSION}.tar.gz}
ENV PKG_CONFIG_PATH=/usr/local/lib/pkgconfig

ARG VERSION=0.0.0
ENV VERSION=$VERSION
ARG TARGETARCH
ENV PLATFORM=$TARGETARCH
RUN .github/scripts/compile.sh
RUN mkdir -p /image && if [ "$PLATFORM" = "amd64" ]; then tar zxpf /qpid-proton-image.tar.gz -C /image && tar zxpf /skupper-router-image.tar.gz -C /image && tar zxpf /libwebsockets-image.tar.gz -C /image && tar zxpf /libunwind-image.tar.gz -C /image; fi
RUN if [ "$PLATFORM" = "arm64" ]; then tar zxpf /qpid-proton-image.tar.gz -C /image && tar zxpf /skupper-router-image.tar.gz -C /image && tar zxpf /libwebsockets-image.tar.gz -C /image; fi
RUN if [ "$PLATFORM" = "s390x" ]; then tar zxpf /qpid-proton-image.tar.gz -C /image && tar zxpf /skupper-router-image.tar.gz -C /image && tar zxpf /libwebsockets-image.tar.gz -C /image; fi
RUN if [ "$PLATFORM" = "ppc64le" ]; then tar zxpf /qpid-proton-image.tar.gz -C /image && tar zxpf /skupper-router-image.tar.gz -C /image && tar zxpf /libwebsockets-image.tar.gz -C /image; fi

RUN mkdir /image/licenses && cp ./LICENSE /image/licenses

# registry.access.redhat.com/ubi9/ubi:latest — pin manifest list digest
FROM registry.access.redhat.com/ubi9/ubi@sha256:5426a8f45e80a07168a30ea24d84f266094b3756624a5508cc53927e6ee39e09 AS packager

RUN dnf -y --setopt=install_weak_deps=0 --nodocs \
    --installroot /output install \
    coreutils-single \
    cyrus-sasl-lib cyrus-sasl-plain openssl \
    python3 \
    libnghttp2 \
    hostname iputils \
    shadow-utils \
 && chroot /output useradd --uid 10000 runner \
 && dnf -y --installroot /output remove shadow-utils \
 && dnf clean all --installroot /output
RUN [ -d /usr/share/buildinfo ] && cp -a /usr/share/buildinfo /output/usr/share/buildinfo ||:
RUN [ -d /root/buildinfo ] && cp -a /root/buildinfo /output/root/buildinfo ||:

# golang:1.26.6-alpine — pin manifest list digest
FROM golang:1.26.6-alpine@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83 AS go-builder

ARG TARGETOS
ARG TARGETARCH

RUN mkdir -p /go/src/github.com/eclipse-iofog/router
WORKDIR /go/src/github.com/eclipse-iofog/router
COPY . /go/src/github.com/eclipse-iofog/router
RUN go fmt ./...
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath  -ldflags="-s -w" -o bin/router .

# registry.access.redhat.com/ubi9/ubi-minimal:latest — pin manifest list digest
FROM registry.access.redhat.com/ubi9/ubi-minimal@sha256:8eb2830d0936237fc13a1f2f7e45aecf90d69043380ad167fad0343632937f41 AS tz
RUN microdnf install -y tzdata && microdnf reinstall -y tzdata

FROM scratch

ARG OCI_SOURCE_REPO
ARG OCI_VERSION=latest
ARG OCI_REVISION
ARG ROUTER_DISTRIBUTION

LABEL org.opencontainers.image.source="${OCI_SOURCE_REPO}" \
      org.opencontainers.image.version="${OCI_VERSION}" \
      org.opencontainers.image.revision="${OCI_REVISION}" \
      distribution="${ROUTER_DISTRIBUTION}"

COPY --from=packager /output /
COPY --from=packager /etc/yum.repos.d /etc/yum.repos.d

USER 10000

COPY --from=builder /image /

WORKDIR /home/skrouterd/bin
COPY scripts/launch.sh /home/skrouterd/bin/launch.sh

ENV VERSION=${OCI_VERSION}
ENV QDROUTERD_HOME=/home/skrouterd

COPY LICENSE /licenses/LICENSE
COPY --from=go-builder /go/src/github.com/eclipse-iofog/router/bin/router /home/skrouterd/bin/router

COPY --from=tz /usr/share/zoneinfo /usr/share/zoneinfo

# Env: SKUPPER_PLATFORM=pot|iofog|kubernetes (default pot), QDROUTERD_CONF (default /tmp/skrouterd.json),
# SSL_PROFILE_PATH (default /etc/skupper-router-certs), EDGELET_MICROSERVICE_UID (required in pot mode), SSL=true|false.
# Pot mode uses ioFog LocalAPI v3 over HTTPS/WSS with service-account token and CA mounts.
# In K8s mode operator mounts config at QDROUTERD_CONF.
CMD ["/home/skrouterd/bin/router"]