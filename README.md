# fileconduit v0.0.2

`fileconduit` is a client/server application that aids to transfer files securely between two systems that access the internet but don't access each other.

You install the server component (via docker) on a third server, expose it (e.g. securely via a reverse proxy) and execute a script on the source system:

```bash
python3 uploader.py myfile.bin
```

This will print a secure link to download the file from, using a browser or `curl` or however you want.

`fileconduit` **transfers single files**: you can upload several files concurrently, with repeated uploader.py executions. One download is possible for each, and the uploader script will exit after successful download.

# Quickstart/demo

For a quick test of how it works, you can run it locally. Prerequisites are `docker` and `python` v3, a file to upload, nothing else.

Run the server:

```bash
docker run --rm -p 8080:8080 -e FILECONDUIT_SECRET_HASH=652c7dc687d98c9889304ed2e408c74b611e86a40caa51c4b43f1dd5913c5cd0 germanorizzo/fileconduit:latest
```

Then download `uploader.py` from this repository and run it in another console:

```bash
python3 uploader.py myfile.bin
```

And follow the instructions to download the file.

# Installation/usage

This section expands on the previous, to explain how to setup `fileconduit` in a proper architecture. It assumes a certain familiarity with `docker`, we won't explain all the concepts involved.

## Server

It's a Java (21+) application, the best way is to install via docker.

Get a server, ideally already provisioned with a reverse proxy. `fileconduit` is best not exposed directly to internet, mainly because it doesn't provide HTTPS.

Generate a secret, best a long (24+) sequence of letters and numbers (to avoid escaping problems), and hash it with SHA256 using for example [this site](https://emn178.github.io/online-tools/sha256.html) that, at time of writing, doesn't seem to send your secret over the intenet (check!).

```bash
docker run --name fileconduit -p 8080:8080 -e FILECONDUIT_SECRET_HASH=<secret_hash> germanorizzo/fileconduit:latest
```

Or, via docker compose:

```
services:
  fileconduit:
    image: germanorizzo/fileconduit:latest
    container_name: fileconduit
    environment:
      - FILECONDUIT_SECRET_HASH=<secret_hash>
    ports:
      - 8080:8080
```

> **Note**: this will expose it on port 8080; if installing with a reverse proxy, you may want to use a docker network.

### Example: using `caddy` as a reverse proxy

This is an excerpt of a `caddyfile`:

```
conduit.example.com {
  reverse_proxy localhost:8080
}
```

## Upload client

Download the file `upload.py` from this repository. Configure it:

- **Line 24**: the secret
- **Line 27**: the base URL that you exposed to internet (in the `caddy` example above, `https://conduit.example.com`)
- **Line 31**: (optional) the buffer size

Then just launch it:

```bash
python3 uploader.py myfile.bin
```

This will output a link with the instructions to download. The link is unique and, while public, it's quite difficult to guess.

```
== fileconduit v0.0.2 ==
All set up! Download your file:
- a browser, from https://pipe.gercloud.cc/dl/4980907730449368564
- a shell, with $> curl -OJ https://pipe.gercloud.cc/dl/4980907730449368564
```

After a client initiates a download and the uploader sends all the data, the uploader script will exit.

# Building the server

In the gradle setup of this repository, use `gradle buildDocker`. This will generate a docker image tagged as `fileconduit:v0.0.2`.

`docker` and `docker buildx` must be properly installed and available.