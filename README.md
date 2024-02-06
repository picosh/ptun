# ptun - web tunnels

Passwordless authentication for the browser using SSH tunnels.

# How it works

The end-user creates a local forward SSH tunnel to a service running `ptun`.
`ptun` spins up a dedicated web service for that tunnel -- using a unix socket.
Then the user can access that web service using `localhost`. The web service can
then access the SSH context in order to know who the user is and what they are
authorized to do within the web service.

# Why?

Sometimes all you have is an ssh keypair for authenticating a user and don't
want to require them to create a completely separate auth mechanism for website
access.

For example, have you ever wished you could use `docker push` and `docker pull`
using just an SSH keypair? Well now it's possible.

# Using with Github Action

Need registry access to CI/CD?  Use our container service to open a "local"
registry.

```yml
name: build and push docker image

on:
  push:
    branches: [main]

jobs:
  build:
    runs-on: ubuntu-latest
    # start ssh tunnel as a container service
    services:
      registry:
        image: ghcr.io/picosh/ptun/autossh:latest
        env:
          USERNAME: <pico_user>
          PRIVATE_KEY: ${{ secrets.PRIVATE_KEY }}
        ports:
          - 5000:5000
    steps:
    - name: Set up QEMU
      uses: docker/setup-qemu-action@v3
    - name: Set up Docker Buildx
      uses: docker/setup-buildx-action@v3
      with:
        driver-opts: network=host
    - name: Build and push
      uses: docker/build-push-action@v5
      with:
        push: true
        tags: localhost:5000/image:latest
```

# Development

```bash
make example
./build/example
```

```bash
make tunnel
```

Go to http://localhost:8443
