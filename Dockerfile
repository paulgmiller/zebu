FROM golang as build-env
RUN mkdir /build 
RUN mkdir /static 
WORKDIR /build 
ADD go.* /build/
RUN go mod download
ADD . /build/
RUN go build -o /zebu ./cmd
FROM gcr.io/distroless/base
COPY --from=build-env /zebu /
ENTRYPOINT ["/zebu"]
