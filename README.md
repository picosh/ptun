# Web Tunnels

A passwordless authentication experience for the browser leveraging SSH tunnels.

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

# Development

```bash
make example
./build/example
```

```bash
make tunnel
```

Go to http://localhost:8081
