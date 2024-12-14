# fileconduit v0.3.1

`fileconduit` is a client/server application that aids to transfer files securely between two systems that access the
internet but don't access each other.

You install the server component (via docker) on a third server, expose it (e.g. securely via a reverse proxy) and
execute a script on the source system:

```bash
python3 fcuploader.py myfile.bin
```

This will print a secure link to download the file from, using a browser or `curl` or however you want.

`fileconduit` **transfers single files**: you can upload several files concurrently, with repeated `fcuploader.py`
executions. One download is possible for each, and the fcuploader script will exit after successful download.

# Quickstart/demo

For a quick test of how it works, you can run it locally. Prerequisites are `docker` and `python` v3, a file to
upload, nothing else.

Run the server:

```bash
docker run --rm -p 8080:8080 -e FILECONDUIT_SECRET_HASHES=652c7dc687d98c9889304ed2e408c74b611e86a40caa51c4b43f1dd5913c5cd0 germanorizzo/fileconduit:latest
```

Then download `fcuploader.py` from this repository and run it in another console:

```bash
python3 fcuploader.py myfile.bin
```

And follow the instructions to download the file.

# Installation/usage

This section expands on the previous, to explain how to set up `fileconduit` in a proper architecture. It assumes a
certain familiarity with `docker`, we won't explain all the concepts involved.

## Server

It's a Go application but it's tailor-made to be configured and installed via Docker.

Get a server, ideally already provisioned with a reverse proxy. `fileconduit` is best not exposed directly to internet,
mainly because it doesn't provide HTTPS.

Generate a secret, best a long (24+) sequence of letters and numbers (to avoid escaping problems), and hash it with
SHA256 using for example [this site](https://emn178.github.io/online-tools/sha256.html) that, at time of writing, doesn't seem to send your secret over the intenet
(check!).

> You can generate several hashes, and specify them as a comma-separated list.

```bash
docker run --name fileconduit -p 8080:8080 -e FILECONDUIT_SECRET_HASHES=<secret_hash[,<another_one>,...]> germanorizzo/fileconduit:latest
```

Or, via docker compose:

```
services:
  fileconduit:
    image: germanorizzo/fileconduit:latest
    container_name: fileconduit
    environment:
      - FILECONDUIT_SECRET_HASHES=<secret_hash[,<another_one>,...]>
    ports:
      - 8080:8080
```

> This will expose it on port 8080; if installing with a reverse proxy, you may want to set up a docker network. You can
> set `internal: true` on it, `fileconduit` doesn't need to access any other system other than the reverse proxy.  

### Example: using `caddy` as a reverse proxy

This is an excerpt of a `caddyfile`:

```
conduit.example.com {
  reverse_proxy localhost:8080
}
```

## Upload client

Download the file `upload.py` from this repository.

Configure it with the secret and the base URL that you exposed to internet (in the `caddy` example above,
`https://conduit.example.com`)

Then just launch it:

```bash
python3 fcuploader.py myfile.bin
```

This will output a link with the instructions to download. The link is unique and, while public, it's quite difficult
to guess.

```
== fileconduit v0.3.1 ==
All set up! Download your file using:
- a browser, from https://conduit.example.com/dl/I5zeoJIId1d10FAvnsJrp4q6I2f2F3v7j
- a shell, with $> curl -OJ https://conduit.example.com/dl/I5zeoJIId1d10FAvnsJrp4q6I2f2F3v7j
```

After a client initiates a download and the fcuploader sends all the data, the fcuploader script will exit.

# Building the server

In the root dir of this repository, use `docker buildx build . -t fileconduit:v0.3.1`. This will generate a docker image
tagged as `fileconduit:v0.3.1`.

`docker` and `docker buildx` must be properly installed and available.
