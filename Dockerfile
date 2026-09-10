# Start from the official Go image to ensure we have a full Go environment.
FROM golang:1.26 AS builder

# Set the working directory inside the container.
WORKDIR /app

# Copy the go.mod and go.sum to download all dependencies.
COPY go.mod .
COPY go.sum .

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed.
RUN go mod download -x

# Copy the source code into the container.
COPY . .

# Build the application. This will create a binary with the name "server".
RUN CGO_ENABLED=0 GOOS=linux go build -v -o server

# Use a Docker multi-stage build to create a lean production image.
# Start from the alpine image to have a minimal base image.
FROM alpine:latest  

# Set the working directory in the new container.
WORKDIR /root/

# Copy the binary from the builder stage.
COPY --from=builder /app/server .

# Expose the port the server listens on.
EXPOSE 8080

# Command to run the binary.
CMD ["./server"]
