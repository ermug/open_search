FROM golang:1.24-alpine

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy all necessary source files
COPY main.go ./
COPY indexer.go ./
COPY text_processor.go ./

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o indexer

# Run
CMD ["./indexer"] 