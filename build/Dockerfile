FROM golang:1.24-alpine
WORKDIR /app

RUN apk add --no-cache git
RUN go install github.com/air-verse/air@latest
RUN go install github.com/go-delve/delve/cmd/dlv@latest

# Correctly copy from the build context (project root)
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire project
COPY ../. .

EXPOSE 9999
EXPOSE 2345

CMD ["air"]