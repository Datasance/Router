FROM golang:1.20.14-alpine AS go-builder

ARG TARGETOS
ARG TARGETARCH

RUN mkdir -p /go/src/github.com/datasance/router
WORKDIR /go/src/github.com/datasance/router
COPY . /go/src/github.com/datasance/router
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o bin/router



FROM quay.io/skupper/skupper-router:3.0.1
COPY LICENSE /licenses/LICENSE
COPY --from=go-builder /go/src/github.com/datasance/router/bin/router /home/skrouterd/bin/router
COPY scripts/launch.sh /home/skrouterd/bin/launch.sh

CMD ["/home/skrouterd/bin/router"]
