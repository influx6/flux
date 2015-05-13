package flux

import "testing"

func TestBlockPushSocket(t *testing.T) {
	sock := PushSocket(0)

	sock.Subscribe(func(v interface{}, r *Sub) {
		// defer r.Close()
		_, ok := v.(string)
		t.Log("blockpush:", v)
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
		// defer r.Close()
		t.Log("testpusher:", v)
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

func TestConditionedPushPullSocket(t *testing.T) {
	sock := PushSocket(0)
	dsock := DoPushSocket(sock, func(v interface{}, s SocketInterface) {
		if v == "Bottle" {
			s.Emit(v)
		}
	})

	defer dsock.Close()
	defer sock.Close()

	dsock.Subscribe(func(v interface{}, s *Sub) {
		// defer s.Close()
		t.Log("cond:", v)
		_, ok := v.(string)
		if !ok {
			t.Fatal("value received is not a string", v, ok, s)
		}
	})

	sock.Emit("Token")
	sock.Emit("Bottle")
	sock.Emit("Beer")

}
