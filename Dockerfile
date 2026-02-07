FROM golang:1.24-alpine

WORKDIR /app

# Install essential build tools (git is often needed for go mod download if dependencies are private or complex)
RUN apk add --no-cache git

# Install air for hot reload
RUN go install github.com/air-verse/air@v1.62.0

# Install templ
RUN go install github.com/a-h/templ/cmd/templ@v0.3.977

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Expose port
EXPOSE 8080

# Command to run air
CMD ["air", "-c", ".air.toml"]
