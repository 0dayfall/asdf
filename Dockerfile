# Start from a Debian-based image with Go installed
FROM golang:latest

# Set the working directory inside the container
WORKDIR /app

# Copy the local package files to the container's workspace
COPY . .

# Build the Go app
RUN go build -o main ./cmd

# Expose the port (this is for documentation purposes)
EXPOSE $PORT

# Run the binary program produced by `go install`
CMD ["/app/main"]   
