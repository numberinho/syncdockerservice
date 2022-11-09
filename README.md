# syncdockerservice

Small Go webserver, listening for docker.hub webhook to update or deploy images.

Caution: Docker-socket is exposed to container, may cause security issues.

Edit config.json for your containers.

Build image: <br>
```
docker build -t syncdocker:latest .
```

Run image: <br>
```
docker run --rm -it -p 3333:3333 -v /var/run/docker.sock:/var/run/docker.sock \
    -e ENV_WEBHOOK_TOKEN="" \
    -e ENV_SERVICESYNC_PORT="3333" \
    -e ENV_USERNAME="" \
    -e ENV_PASSWORD="" \
--platform linux/amd64 --name syncdocker  ligainsider/syncdocker:latest
```

```ENV_PASSWORd:``` Docker Hub password<br>
```ENV_USERNAME:``` Docker Hub username<br>
```ENV_WEBHOOK_TOKEN:``` Token, specify in docker hub<br>
```ENV_SERVICESYNC_PORT:``` Port for the webserver<br>
