package wstest

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"

	"github.com/gorilla/websocket"
)

// Client is a websocket client for unit testing
type Client struct {
	httptest.ResponseRecorder
	*websocket.Conn
	sConn *conn
	cConn *conn
}

// NewClient returns a new client
func NewClient() *Client {
	sConn, cConn := newPairedConnections()
	return &Client{
		sConn: sConn,
		cConn: cConn,
	}
}

// WithLogger sets debug logging for the client and connections
// log is a Println-like function
func (c *Client) WithLogger(log log) *Client {
	c.sConn.Log = log
	c.cConn.Log = log
	return c
}

// Connect a wstest Client to an http.Handler which accepts websocket upgrades.
// This send an HTTP request to the http.Handler, and wait for the connection upgrade response.
// it uses the gorilla's websocket.Dial function, over a fake net.Conn struct.
// it runs the server's ServeHTTP function in a goroutine, so server can communicate with a
// client running on the current program flow
// h is the handler that should handle websocket connections.
// url is the url to connect to that handler. the host and port are not important, but protocol
// should be ws or wss, and the path should be the one that expects websocket connections
func (c *Client) Connect(h http.Handler, url string) error {
	var err error

	// run the runServer in a goroutine, so when the Dial send the request to
	// the server on the connection, it will be parsed as an HTTPRequest and
	// sent to the Handler function.
	go c.runServer(h)

	// use the websocket.Dialer.Dial with the fake net.Conn to communicate with the server
	// the dialer gets the cConn which is the client side of the connection
	dialer := &websocket.Dialer{NetDial: func(network, addr string) (net.Conn, error) { return c.cConn, nil }}
	c.Conn, _, err = dialer.Dial(url, nil)
	if err != nil {
		return err
	}
	return nil
}

// dialer handler reads the request sent on the connection to the server
// from the websocket.Dialer.Dial function, and pass it to the server.
// once this is done, the communication is done on the wsConn
func (c *Client) runServer(h http.Handler) {
	// read from the server connection the request sent by the dialer.Dial,
	// and use the handler to serve this request.
	req, err := http.ReadRequest(bufio.NewReader(c.sConn))
	if err != nil {
		panic(err)
	}
	h.ServeHTTP(c, req)
}

// Hijack the connection
func (c *Client) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	// return to the server the sConn, which is the server side of the connection
	rw := bufio.NewReadWriter(bufio.NewReader(c.sConn), bufio.NewWriter(c.sConn))
	return c.sConn, rw, nil
}

// WriteHeader write HTTP header to the client and closes the connection
func (c *Client) WriteHeader(code int) {
	r := http.Response{StatusCode: code}
	r.Write(c.sConn)
	c.sConn.Close()
}

// Close closes the websocket connection
func (c *Client) Close() error {
	if c.Conn == nil {
		return nil
	}
	return c.Conn.Close()
}
