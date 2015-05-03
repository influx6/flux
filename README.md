# flux
FBP style socket structure which provide simple data buffering and listener notification

##Sockets
 - BufferPush
 builds on top of the socket structure where every data emitted is immediately sent to the listener lists or buffers within the set size of its buffered channel until a listener is added and the 'Pull' method is called

 - Push
 builds on top of the socket structure where every data emitted is immediately sent to the listener lists

 - Pull
 builds on top of the socket structure and emits data into a buffered channel which then can be pull into the listeners


#Examples

- Pull Sockets
```

	sock := PullSocket(10)

	defer sock.Close()

	sock.Subscribe(func(v interface{}, s *Sub) {
		defer s.Close()
		_, ok := v.(string)
		if !ok {
			t.Fatal("value received is not a string", v, ok, s)
		}
	})

	sock.Emit("Token")
	sock.Emit("Bottle")
	sock.Pull()


```

- Push Sockets

```

	sock := PushSocket(10)

	defer sock.Close()

	sock.Subscribe(func(v interface{}, s *Sub) {
		defer s.Close()
		_, ok := v.(string)
		if !ok {
			t.Fatal("value received is not a string", v, ok, s)
		}
	})

	sock.Emit("Token")
	sock.Emit("Bottle")

```

- PushPull Sockets

```

	sock := PushSocket(10)
	dsock := DoPullSocket(sock, func(v interface{}, s SocketInterface) {
		if v == "Bottle" {
			s.Emit(v)
		}
	})

	defer sock.Close()
	defer dsock.Close()

	dsock.Subscribe(func(v interface{}, s *Sub) {
		defer s.Close()
		_, ok := v.(string)
		if !ok {
			t.Fatal("value received is not a string", v, ok, s)
		}
	})

	sock.Emit("Token")
	sock.Emit("Bottle")
	sock.Emit("Beer")
	dsock.Pull()

```


- PushPull Sockets

```

	sock := PullSocket(10)
	dsock := DoPushSocket(sock, func(v interface{}, s SocketInterface) {
		if v == "Bottle" {
			s.Emit(v)
		}
	})

	defer sock.Close()
	defer dsock.Close()

	dsock.Subscribe(func(v interface{}, s *Sub) {
		defer s.Close()
		_, ok := v.(string)
		if !ok {
			t.Fatal("value received is not a string", v, ok, s)
		}
	})

	sock.Emit("Token")
	sock.Emit("Bottle")
	sock.Emit("Beer")
	sock.Pull()


```
