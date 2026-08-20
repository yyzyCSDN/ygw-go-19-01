# Container verification

The image uses the official Go 1.23 base, retains the full toolchain, downloads
dependencies during build, and then sets `GOPROXY=off` and `GOSUMDB=off`.

```sh
docker build -f benzhi.Dockerfile -t ygw-go-09-06:benzhi .
docker run --rm ygw-go-09-06:benzhi go test ./...
```

The build script defaults to amd64. Set `TARGET_PLATFORM=linux/arm64` for arm64.
