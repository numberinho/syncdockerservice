# syncdockerservice

Small Go webserver, listening for docker.hub webhook to update the image of a running service.

Caution: Docker-socket is exposed to container, may cause security issues.


Build image: <br>
```
docker build -t syncdocker:latest .
```

Run image: <br>
```
docker run --rm -d -p 3333:3333 -v /var/run/docker.sock:/var/run/docker.sock \
    -e ENV_REPO="user/image" \
    -e ENV_TAG="latest" \
    -e ENV_SERVICE_ID="SERVICE_ID" \
    -e ENV_WEBHOOK_TOKEN="WEBHOOK_TOKEN" \
    -e ENV_SERVICESYNC_PORT="3333" \
    syncdocker:latest
```

```ENV_REPO:``` Repository to watch (Image) <br>
```ENV_TAG:``` Only watch matching tag<br>
```ENV_SERVICE_ID:``` Service to update<br>
```ENV_WEBHOOK_TOKEN:``` Token, specify in docker hub<br>
```ENV_SERVICESYNC_PORT:``` Port for the webserver<br>
