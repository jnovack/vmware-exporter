FROM golang:1.14 as builder
WORKDIR /go/src/app

# Create appuser.
# See https://stackoverflow.com/a/55757473/12429735RUN
ENV USER=appuser
ENV UID=10001
RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    "${USER}"

RUN apk add --no-cache git ca-certificates

COPY go.mod .
COPY go.sum .
RUN go mod download

ARG APPLICATION="go-executable"
ARG BUILD_RFC3339="1970-01-01T00:00:00Z"
ARG COMMIT="local"
ARG VERSION="dirty"
ARG GO_LDFLAGS="-w -s \
        -X github.com/jnovack/go-version.Application=${APPLICATION} \
        -X github.com/jnovack/go-version.BuildDate=${BUILD_RFC3339} \
        -X github.com/jnovack/go-version.Revision=${COMMIT} \
        -X github.com/jnovack/go-version.Version=${VERSION} \
        -extldflags '-static'"

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "${GO_LDFLAGS}" -o /go/bin/${APPLICATION} .

#######################################################################
FROM scratch

WORKDIR /app

ARG BUILD_RFC3339="1970-01-01T00:00:00Z"
ARG COMMIT="local"
ARG VERSION="dirty"

# Port as decided by prometheus exporters list
EXPOSE 9094

LABEL org.opencontainers.image.ref.name="jnovack/vmware-exporter" \
      org.opencontainers.image.created=$BUILD_RFC3339 \
      org.opencontainers.image.authors="Justin J. Novack <jnovack@gmail.com>" \
      org.opencontainers.image.documentation="https://github.com/jnovack/vmware-exporter/README.md" \
      org.opencontainers.image.description="Minimalist vmware-exporter for ESXi hosts or vCenter deployments." \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.source="https://github.com/jnovack/vmware-exporter" \
      org.opencontainers.image.revision=$COMMIT \
      org.opencontainers.image.version=$VERSION \
      org.opencontainers.image.url="https://hub.docker.com/r/jnovack/vmware-exporter/"


COPY --from=builder /go/bin/vmware-exporter .
COPY config.properties .

CMD ["./vmware-exporter"]