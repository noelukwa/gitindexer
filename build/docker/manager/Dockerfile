FROM golang:1.22 as builder

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o explorer cmd/explorer/main.go

FROM alpine:latest

WORKDIR /root/explorer

COPY --from=builder /build/explorer .

CMD ["./explorer"]
