# Start from golang v1.12 base image
FROM golang:1.14 as builder

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy dependencies lock files and install
COPY go.mod .
COPY go.sum .
RUN go mod download

# Build the Go app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /go/bin/vmware-exporter .

######## Start a new stage from scratch #######
FROM scratch

WORKDIR /app

# Port as decided by prometheus exporters list
EXPOSE 9094

COPY --from=builder /go/bin/vmware-exporter .
COPY config.properties .

CMD ["./vmware-exporter"]