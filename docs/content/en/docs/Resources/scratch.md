---
title: "Images from scratch"
linkTitle: "ScratchImages"
weight: 4
description: >
  Using Luet to compose images from scratch
---

The Docker image `quay.io/luet/base` is a `scratch` Docker image always kept up-to-date with the latest luet version. That image can be used to bootstrap new images with Luet repositories with the packages you want, from the repositories you prefer.

For example we can mount a config file, and later on install a package:

```bash
cat <<EOF > $PWD/luet.yaml  
repositories: 
  - name: "micro-stable"
    enable: true
    cached: true
    priority: 1
    type: "http"
    urls: 
    - "https://get.mocaccino.org/mocaccino-micro-stable"
EOF

docker rm luet-runtime-test || true
docker run --name luet-runtime-test \
       -ti -v /tmp:/tmp \
       -v $PWD/luet.yaml:/etc/luet/luet.yaml:ro \
       quay.io/luet/base install shells/bash
 
docker commit luet-runtime-test luet-runtime-test-image

# Try your new image!

docker run -ti --entrypoint /bin/bash --rm luet-runtime-test-image
```

In this way we will create a new image, with only `luet` and `bash`, and nothing else from a scratch image.
