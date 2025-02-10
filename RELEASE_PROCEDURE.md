# Release procedure

## Update libraries

```bash
cd src
go get -u
go mod tidy
```

Check latest version of Go, and in case it change update `go.mod`.

## Builder image

Get the docker image hash that will build the binary:

```bash
docker pull golang:latest
docker images --digests | grep golang | grep latest | awk '{print $3}'
# sha256:927112936d6b496ed95f55f362cc09da6e3e624ef868814c56d55bd7323e0959
```

Replace it in the 3 `Dockerfile.*`:

```dockerfile
FROM golang@<SHA256> AS build
```

## Version

Replace the new version string (e.g. `v0.4.1`) in:

- `README.md`
- `fileway_ul.py`

## Commit and tag

Commit the version, tag it and push everything:

```bash
git add .
git commit -S -m "v0.5.2" # Provide GPG password
git push
git tag -s "v0.5.2" -m "v0.5.2"
git push origin "v0.5.2"
```

The CI pipeline should start.
