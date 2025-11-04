FROM golang:1.23-alpine AS go-builder

ARG TARGETOS
ARG TARGETARCH

RUN mkdir -p /go/src/github.com/datasance/router
WORKDIR /go/src/github.com/datasance/router
COPY . /go/src/github.com/datasance/router
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o bin/router

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest AS tz
RUN microdnf install -y tzdata && microdnf reinstall -y tzdata

FROM quay.io/skupper/skupper-router:main
COPY LICENSE /licenses/LICENSE
COPY --from=go-builder /go/src/github.com/datasance/router/bin/router /home/skrouterd/bin/router
# COPY scripts/launch.sh /home/skrouterd/bin/launch.sh
COPY --from=tz /usr/share/zoneinfo /usr/share/zoneinfo

CMD ["/home/skrouterd/bin/router"]
