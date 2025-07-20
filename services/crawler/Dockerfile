FROM golang:1.24-alpine

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy all necessary source files
COPY main.go ./
COPY text_processor.go ./
COPY indexer.go ./

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o crawler

# Run
CMD ["./crawler"] 