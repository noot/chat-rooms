# chat-rooms
go-libp2p chat app

usage: 

```
go build -o chat
./chat [port] [optional room name]
```

choose any available port to listen on. join an optional room; you will only send and receive messages to others in the same room.

if no room name is specified, it will join `main`.
