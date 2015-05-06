package flux

import "testing"

func TestBufferSocket(t *testing.T) {
	sock := BufferPushSocket(3)

	defer sock.Close()

	sock.Emit("Token")
	sock.Emit("Bottle")

	sb := sock.Subscribe(func(v interface{}, s *Sub) {
		// defer s.Close()
		_, ok := v.(string)
		if !ok {
			t.Fatal("value received is not a string", v, ok, s)
		}
	})

	defer sb.Close()

	sock.PullStream()
	sock.Emit("Throttle")
}

func TestPullSocket(t *testing.T) {
	sock := PullSocket(1)

	defer sock.Close()

	sock.Subscribe(func(v interface{}, s *Sub) {
		defer s.Close()
		_, ok := v.(string)
		if !ok {
			t.Fatal("value received is not a string", v, ok, s)
		}
	})

	sock.Emit("Token")
	sock.PullStream()
	sock.Emit("Bottle")
	sock.PullStream()
}

func TestPushSocket(t *testing.T) {
	sock := PushSocket(1)

	defer sock.Close()

	sock.Subscribe(func(v interface{}, s *Sub) {
		// defer s.Close()
		_, ok := v.(string)
		if !ok {
			t.Fatal("value received is not a string", v, ok, s)
		}
	})

	sock.Emit("Token")
	sock.Emit("Bottle")
}

func TestConditionedPushPullSocket(t *testing.T) {
	sock := PushSocket(3)
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
	dsock.PullStream()

}

func TestConditionedPullPushSocket(t *testing.T) {
	sock := PullSocket(3)
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
	sock.PullStream()

}
