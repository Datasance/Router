FROM registry.access.redhat.com/ubi9/ubi-minimal:latest AS builder

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
# Clone skupper-router so repo contents are in /build (not /build/skupper-router)
RUN git clone --depth 1 --branch main https://github.com/skupperproject/skupper-router.git .
ENV PROTON_VERSION=main
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

FROM registry.access.redhat.com/ubi9/ubi:latest AS packager

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

FROM golang:1.23-alpine AS go-builder

ARG TARGETOS
ARG TARGETARCH

RUN mkdir -p /go/src/github.com/datasance/router
WORKDIR /go/src/github.com/datasance/router
COPY . /go/src/github.com/datasance/router
RUN go fmt ./...
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath  -ldflags="-s -w" -o bin/router .

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest AS tz
RUN microdnf install -y tzdata && microdnf reinstall -y tzdata

FROM scratch

COPY --from=packager /output /
COPY --from=packager /etc/yum.repos.d /etc/yum.repos.d

USER 10000

COPY --from=builder /image /

WORKDIR /home/skrouterd/bin
COPY ./scripts/* /home/skrouterd/bin/

ARG version=latest
ENV VERSION=${version}
ENV QDROUTERD_HOME=/home/skrouterd

COPY LICENSE /licenses/LICENSE
COPY --from=go-builder /go/src/github.com/datasance/router/bin/router /home/skrouterd/bin/router

COPY --from=tz /usr/share/zoneinfo /usr/share/zoneinfo

# Env: SKUPPER_PLATFORM=pot|kubernetes (default pot), QDROUTERD_CONF (default /tmp/skrouterd.json),
# SSL_PROFILE_PATH (default /etc/skupper-router-certs). In K8s mode operator mounts config at QDROUTERD_CONF.
CMD ["/home/skrouterd/bin/router"]