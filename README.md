```bash
make example
./build/example
```

```bash
ssh -L 8081:localhost:3000 -p 2222 -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -N localhost
```

Go to http://localhost:8081
