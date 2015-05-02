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
