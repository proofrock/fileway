FROM golang:latest as build

WORKDIR /go/src/app
COPY src/ .

RUN CGO_ENABLED=0 go build -o fileway -trimpath

# Now copy it into our base image.
FROM gcr.io/distroless/static-debian12
COPY --from=build /go/src/app/fileway /

ENV FILEWAY_SECRET_HASHES=""

EXPOSE 8080

ENTRYPOINT ["/fileway"]
