package flux

import "testing"

func TestBlockPushSocket(t *testing.T) {
	sock := PushSocket(0)

	sock.Subscribe(func(v interface{}, r *Sub) {
		_, ok := v.(string)
		if !ok {
			t.Fatal("value received is not a string", v, ok, r)
		}
	})

	sock.PushStream()
	sock.PushStream()
	sock.PushStream()

	sock.Emit("Token")
	sock.Emit("Bottle")
	sock.Emit("Rocket")

	sock.Close()
}

func TestPushSocket(t *testing.T) {
	sock := PushSocket(3)

	sock.Subscribe(func(v interface{}, r *Sub) {
		_, ok := v.(string)
		if !ok {
			t.Fatal("value received is not a string", v, ok, r)
		}
	})

	sock.PushStream()
	sock.PushStream()
	sock.PushStream()

	sock.Emit("Token")
	sock.Emit("Bottle")
	sock.Emit("Rocket")

	sock.Close()
}

func TestPullSocket(t *testing.T) {
	sock := PullSocket(3)

	sock.Subscribe(func(v interface{}, r *Sub) {
		_, ok := v.(string)
		if !ok {
			t.Fatal("value received is not a string", v, ok, r)
		}
	})

	sock.Emit("Token")
	sock.PushStream()
	sock.Emit("Bottle")
	sock.PushStream()
	sock.Emit("Rocket")
	sock.PushStream()

	sock.Close()
}

func TestConditionedPushSocket(t *testing.T) {
	sock := PushSocket(0)
	dsock := DoPushSocket(sock, func(v interface{}, s SocketInterface) {
		if v == "Bottle" {
			s.Emit(v)
		}
	})

	defer dsock.Close()
	defer sock.Close()

	dsock.Subscribe(func(v interface{}, s *Sub) {
		_, ok := v.(string)
		if !ok {
			t.Fatal("value received is not a string", v, ok, s)
		}
	})

	sock.Emit("Token")
	sock.Emit("Bottle")
	sock.Emit("Beer")

}

func TestConditionedPushPullSocket(t *testing.T) {
	sock := PushSocket(0)
	dsock := DoPullSocket(5, sock, func(v interface{}, s SocketInterface) {
		if v == "Beer" {
			s.Emit(v)
		}
	})

	defer dsock.Close()
	defer sock.Close()

	dsock.Subscribe(func(v interface{}, s *Sub) {
		_, ok := v.(string)
		if !ok {
			t.Fatal("value received is not a string", v, ok, s)
		}
	})

	sock.Emit("Token")
	sock.Emit("Bottle")
	sock.Emit("Beer")
	dsock.PushStream()

}
