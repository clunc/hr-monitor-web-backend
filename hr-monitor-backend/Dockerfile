# Stage 1: Build stage
FROM golang:1.20 as builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o main .
RUN ls -la /app  # Debugging: List files to verify binary creation

# Stage 2: Runtime stage
FROM debian:latest
WORKDIR /app
COPY --from=builder /app/main .
RUN ls -la /app  # Debugging: List files to verify binary copying
RUN chmod +x /app/main  # Ensure the binary is executable
CMD ["./main"]
