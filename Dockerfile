# Start from a Debian-based image with Go installed
FROM golang:latest

# Set the working directory inside the container
WORKDIR /app

# Copy the local package files to the container's workspace
COPY . .

# Copy the data file to the container's workspace
COPY ./data/data.json /data/data.json

# Build the Go app
RUN go build -o main ./cmd/asdf

# Expose env variables (this is for documentation purposes)
EXPOSE $PORT
EXPOSE $SSL_CERT_PATH
EXPOSE $SSL_KEY_PATH
EXPOSE $SESSION_KEY

# Run the binary program produced by `go install`
CMD ["/app/main"]   
