package flux

import "testing"

func TestPullSocket(t *testing.T) {
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
}

func TestPushSocket(t *testing.T) {
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
}

func TestConditionedPushPullSocket(t *testing.T) {
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

}

func TestConditionedPullPushSocket(t *testing.T) {
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

}
