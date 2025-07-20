FROM golang:1.24-alpine

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY query_engine/ ./query_engine/

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o query-engine ./query_engine/cmd/search

# Run
CMD ["./query-engine"] 