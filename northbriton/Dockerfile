FROM golang:alpine as builder
RUN mkdir /build 
WORKDIR /build 
ADD go.* /build/
RUN go mod download
ADD . /build/
WORKDIR /build 
RUN go build -o main .
FROM alpine
RUN adduser -S -D -H -h /app appuser
USER appuser
COPY --from=builder /build/main /app/
WORKDIR /app
CMD ["./main"]