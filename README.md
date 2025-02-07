# 🚠 fileway v0.4.0

`fileway` is a client/server application that accepts an upload of a single file; it blocks the upload until a download is initiated, then processes the upload and sends the data to the downloading client. It can be used to transfer files from a server to another, if the two servers don't easily "see" each other, by being installed to a third server (on the internet) that they both see.

```mermaid
sequenceDiagram
    participant Uploader as Uploader
    participant Fileway as Fileway
    participant Downloader as Downloader

    Uploader->>Fileway: Initiate upload
    Uploader-->>Uploader: Wait for download
    Downloader->>Fileway: Request file
    Uploader->>Fileway: Send file data
    Fileway->>Downloader: Download file
```

The transfer is secure: a unique link is generated, and you should only take care to serve it via HTTPS (see the relevant section below).

Uploads can be done with a web interface or via a python3 script, for shells. Downloads can be done via a browser or using the commandline, e.g. `curl`. The uploading script or web session must be kept online until the transfer is done. Of course, multiple concurrent transfers are possible, and it transfers one file at a time.

`fileway` doesn't store anything on the server, it just keeps a buffer to make transfers smooth. It doesn't have any dependency other than `go`. It's distributed as a docker image, but you can easily build it yourself. Also provided, a docker image that includes `caddy` for simple HTTPS provisioning.

## Quickstart/demo

For a quick test of how it works, you can run it locally. Prerequisites are `docker`, a file to upload, nothing else.

Run the server:

```bash
docker run --rm -p 8080:8080 -e FILEWAY_SECRET_HASHES=652c7dc687d98c9889304ed2e408c74b611e86a40caa51c4b43f1dd5913c5cd0 ghcr.io/proofrock/fileway:latest
```

Then open [http://localhost:8080](http://localhost:8080) to access the web page. Put `mysecret` as the secret, and choose a file. Press the Upload button.

In the two boxes that will be displayed, you'll find an URL to be open directly in a browser; and a `curl` commandline to download the file.

> 💡 You can use anything to download that URL, as long as it supports taking the filename from the `Content-Disposition` header. That's the `-J` switch for `curl` and the `--content-disposition` one for `wget` (still marked experimental).

## Installation/usage

This section expands on the previous, to explain how to set up `fileway` in a proper architecture. It assumes a certain familiarity with `docker`, we won't explain all the concepts involved.

A multi-arch docker image (AMD64 and AARCH64) is available in the 'Packages' section of this repository. The client python script to upload files is available in the 'Releases' section.

### Server

It's a Go application but it's tailor-made to be configured and installed via Docker.

Get a server, ideally already provisioned with a reverse proxy. `fileway` is best not exposed directly to internet, mainly because it doesn't provide HTTPS.

Generate a secret, best a long (24+) sequence of letters and numbers (to avoid escaping problems), and hash it with SHA256 using for example [this site](https://emn178.github.io/online-tools/sha256.html) that, at time of writing, doesn't seem to send your secret over the intenet
(check!).

> 💡 You can generate several hashes, and specify them as a comma-separated list.

```bash
docker run --name fileway -p 8080:8080 -e FILEWAY_SECRET_HASHES=<secret_hash[,<another_one>,...]> ghcr.io/proofrock/fileway:latest
```

Or, via docker compose:

```yaml
services:
  fileway:
    image: ghcr.io/proofrock/fileway:latest
    container_name: fileway
    environment:
      - FILEWAY_SECRET_HASHES=<secret_hash[,<another_one>,...]>
    ports:
      - 8080:8080
```

> 💡 If a docker network is needed, you can set `internal: true` on it so that no outbound access is possible. `fileway` doesn't need to access any other system.

#### Docker image with `caddy`

Also published is a docker image `fileway-caddy`, to provide automatic HTTPS via Let'sEncrypt and `caddy`. The usage is very similar, but you must open the ports 80 and 443 (to caddy) and specify the `BASE_ADDRESS` env var to specify the site you're publishing.

```bash
docker run --name fileway -p 8080:8080 -e BASE_ADDRESS=fileway.example.com -e FILEWAY_SECRET_HASHES=<secret_hash[,<another_one>,...]> ghcr.io/proofrock/fileway-caddy:latest
```

#### Using `caddy` as an external reverse proxy

This is an excerpt of a `caddyfile`:

```caddyfile
fileway.example.com {
  reverse_proxy localhost:8080
}
```

### Upload client

#### Web upload client (via browser)

A simple web client is provided. Access it by calling the "root" address, e.g. `https://fileway.example.com`.

![A screenshot of the Web UI](resources/webui.png)

#### Python upload client

Download the file `fileway_ul.py` from this repository. The script doesn't have any dependency other than python3.

Configure it with the secret and the base URL that you exposed to internet (in the `caddy` example above, `https://fileway.example.com`).

Then just launch it:

```bash
python3 fileway_ul.py myfile.bin
```

This will output a link with the instructions to download. The link is unique and, while public, it's quite difficult
to guess.

```text
== fileway v0.4.0 ==
All set up! Download your file using:
- a browser, from https://fileway.example.com/dl/I5zeoJIId1d10FAvnsJrp4q6I2f2F3v7j
- a shell, with $> curl -OJ https://fileway.example.com/dl/I5zeoJIId1d10FAvnsJrp4q6I2f2F3v7j
```

After a client initiates a download and the `fileway_ul.py` sends all the data, the `fileway_ul.py` script will exit.

## Building the server

In the root dir of this repository, use `docker buildx build . -f Dockerfile.simple -t fileway:v0.4.0`. This will generate a docker image
tagged as `fileway:v0.4.0`.

`docker` and `docker buildx` must be properly installed and available.

## Known issues

- The web UI doesn't work correctly on phones (at least iPhones). When the browser goes in background, it thinks it can send the file, but it gets lost.
