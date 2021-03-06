package wstest

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gorilla/websocket"
)

const count = 20

// TestClient demonstrate the usage of wstest package
func TestClient(t *testing.T) {
	t.Parallel()
	var (
		s = &server{Upgraded: make(chan struct{})}
		c = NewClient().WithLogger(t.Log)
	)

	err := c.Connect(s, "ws://example.org/ws")
	if err != nil {
		t.Fatalf("Failed connecting to s: %s", err)
	}

	<-s.Upgraded

	for i := 0; i < count; i++ {
		msg := fmt.Sprintf("hello, world! %d", i)

		err := c.WriteMessage(websocket.TextMessage, []byte(msg))
		if err != nil {
			t.Fatal(err)
		}

		mT, m, err := s.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}

		if want, got := msg, string(m); want != got {
			t.Errorf("server got %s, want  %s", got, want)
		}
		if want, got := websocket.TextMessage, mT; want != got {
			t.Errorf("message type = %s , want %s", got, want)
		}

		s.WriteMessage(websocket.TextMessage, []byte(msg))
		if err != nil {
			t.Fatal(err)
		}

		mT, m, err = c.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}

		if want, got := msg, string(m); want != got {
			t.Errorf("client got %s, want  %s", got, want)
		}
		if want, got := websocket.TextMessage, mT; want != got {
			t.Errorf("message type = %s , want %s", got, want)
		}
	}

	err = c.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = s.Close()
	if err != nil {
		t.Fatal(err)
	}
}

// TestConcurrent tests concurrent reads and writes from a connection
func TestConcurrent(t *testing.T) {
	t.Parallel()
	var (
		s = &server{Upgraded: make(chan struct{})}
		c = NewClient()
	)

	err := c.Connect(s, "ws://example.org/ws")
	if err != nil {
		t.Fatalf("Failed connecting to s: %s", err)
	}

	<-s.Upgraded

	for _, pair := range []struct{ src, dst *websocket.Conn }{{s.Conn, c.Conn}, {c.Conn, s.Conn}} {
		go func() {
			for i := 0; i < count; i++ {
				pair.src.WriteJSON(i)
			}
		}()

		received := make([]bool, count)

		for i := 0; i < count; i++ {
			var j int
			pair.dst.ReadJSON(&j)

			received[j] = true
		}

		missing := []int{}

		for i := range received {
			if !received[i] {
				missing = append(missing, i)
			}
		}
		if len(missing) > 0 {
			t.Errorf("%s -> %s: Did not received: %v", pair.src, pair.dst, missing)
		}
	}

	err = c.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = s.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestBadAddress(t *testing.T) {
	t.Parallel()

	for _, url := range []string{
		"ws://example.org/not-ws",
		"http://example.org/ws",
	} {
		t.Run(url, func(t *testing.T) {
			s := &server{Upgraded: make(chan struct{})}
			c := NewClient()

			err := c.Connect(s, url)
			if err == nil {
				t.Errorf("got unexpected error: %s", err)
			}

			err = c.Close()
			if err != nil {
				t.Fatal(err)
			}

			err = s.Close()
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

// server for test purposes, can't handle multiple websocket connections concurrently
type server struct {
	*websocket.Conn
	upgrader websocket.Upgrader
	Upgraded chan struct{}
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error

	switch r.URL.Path {
	case "/ws":
		s.Conn, err = s.upgrader.Upgrade(w, r, nil)
		if err != nil {
			panic(err)
		}
		close(s.Upgraded)

	default:
		w.WriteHeader(http.StatusNotFound)
	}

}

func (s *server) Close() error {
	if s.Conn == nil {
		return nil
	}
	return s.Conn.Close()
}
