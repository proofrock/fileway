FROM golang:latest as build

WORKDIR /go/src/app
COPY src/ .

RUN CGO_ENABLED=0 go build -o fileconduit -trimpath

# Now copy it into our base image.
FROM gcr.io/distroless/static-debian12
COPY --from=build /go/src/app/fileconduit /

ENV FILECONDUIT_SECRET_HASHES=""

EXPOSE 8080

ENTRYPOINT ["/fileconduit"]
