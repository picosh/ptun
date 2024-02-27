# ptun - web tunnels

Passwordless authentication for the browser using SSH tunnels.

# Demo

We use this library to support private sites through [pgs.sh](https://pgs.sh).

Open a tunnel to pgs:

```bash
ssh -L 5000:localhost:80 -N hey-tunnels@pgs.sh
```

Then go to http://localhost:5000

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

We built this library to support [imgs.sh](https://pico.sh/imgs): a private
docker registry leveraging SSH tunnels.

# Development

```bash
make example
./build/example
```

```bash
make tunnel
```

Go to http://localhost:5000
