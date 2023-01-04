FROM golang as builder
RUN mkdir /build 
RUN mkdir /static 
WORKDIR /build 
ADD go.* /build/
RUN go mod download
ADD . /build/
RUN go build -o /zebu ./cmd
WORKDIR / 
ENTRYPOINT ["/zebu"]
#FROM alpine
#RUN adduser -S -D -H -h /app appuser
#USER appuser
#COPY --from=builder /build/zebu /app/
#WORKDIR /app
#EXPOSE 9000
#CMD ["./zebu"]