# Building

```
go mod vendor
go build -mod vendor ./cmd/update-deployment-secrets/
go build -mod vendor ./cmd/image/
```

# Running

```
WATCH_NAMESPACE=default ./update-deployment-secrets
WATCH_NAMESPACE=default ./image
```
