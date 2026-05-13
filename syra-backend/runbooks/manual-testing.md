# Runbook: Manual Testing

Primary guide: [`MANUAL_TESTING.md`](../MANUAL_TESTING.md).

Quick checks:

```sh
curl -i http://localhost:8080/healthz
curl -i http://localhost:8080/readyz
curl -i http://localhost:8081/healthz
curl -i http://localhost:8081/readyz
```

Load tests:

```sh
chmod +x scripts/load/rest_load.sh scripts/load/rest_to_iso8583_load.sh
API_KEY='<tenant-api-key>' URL='http://localhost:8080/accounts' scripts/load/rest_load.sh
API_KEY='<tenant-api-key>' URL='http://localhost:8080/cards/authorization' scripts/load/rest_to_iso8583_load.sh
```
