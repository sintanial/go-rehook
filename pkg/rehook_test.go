package rehook

import (
	"net"
	"net/http"
	"testing"
	"time"
)

func listener(t *testing.T) net.Listener {
	l, err := net.Listen("tcp", "")
	if err != nil {
		t.Fatal(err)
	}

	return l
}

func receiver(t *testing.T, l net.Listener) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rehook/test", func(w http.ResponseWriter, r *http.Request) {

	})

	if err := http.Serve(l, mux); err != nil {
		t.Fatal(err)
	}
}

func server(t *testing.T, l net.Listener) {
	serverListener := listener(t)
	server := NewServer(nil, 1*time.Second)

	if err := server.Serve(serverListener, http.NewServeMux()); err != nil {
		t.Fatal(err)
	}
}

func client(t *testing.T, receiver net.Listener, server net.Listener) {
	client := NewClient(nil, 0, map[string]string{
		"foorbar": "http://" + receiver.Addr().String() + "/rehook/test",
	})

	client.Tunnel(server.Addr().String())
}

func TestServerClient(t *testing.T) {
	receiverListener := listener(t)
	go receiver(t, receiverListener)

	serverListener := listener(t)
	go server(t, serverListener)

	go client(t, receiverListener, serverListener)

	// todo: add tests
}
