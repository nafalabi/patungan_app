# --- Stage 1: Build CSS ---
FROM node:20-alpine AS css-builder

WORKDIR /app

# Copy package.json and install dependencies
COPY package.json package-lock.json* ./
RUN npm ci

# Copy tailwind config and the raw CSS files
COPY tailwind.config.js ./
COPY web/static/css/ ./web/static/css/
COPY web/templates/ ./web/templates/

# Build minified CSS
RUN npx tailwindcss -i ./web/static/css/input.css -o ./web/static/css/tailwind.css --minify

# --- Stage 2: Build Go binaries ---
FROM golang:1.24-alpine AS go-builder

WORKDIR /app

# Install git for downloading dependencies
RUN apk add --no-cache git

# Copy dependency graphs
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire source code (we assume templ files are pre-generated and tracked in Git)
COPY . .

# Build the Web API Server
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /app/server cmd/server/main.go

# Build the Background Worker
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /app/worker cmd/worker/main.go

# --- Stage 3: Runtime ---
FROM alpine:latest

WORKDIR /app

# Install tzdata and ca-certificates for standard operation
RUN apk add --no-cache ca-certificates tzdata

# Copy built binaries from the go stages
COPY --from=go-builder /app/server .
COPY --from=go-builder /app/worker .

# Copy static assets and specifically the minified CSS from the CSS stage
COPY web/static/ ./web/static/
COPY --from=css-builder /app/web/static/css/tailwind.css ./web/static/css/tailwind.css

# Make binaries executable
RUN chmod +x ./server ./worker

# Port mapped out via Echo
EXPOSE 8080

# Default container entrypoint is the web server API
CMD ["./server"]
