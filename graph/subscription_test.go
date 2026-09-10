package graph_test

import (
	"context"
	"encoding/json"
	"graphql-go/graph"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	websocket "github.com/coder/websocket"
)

// Subscriptions moved from a gorilla Upgrader to gqlgen's pluggable websocket
// implementation in v0.17.95. This drives a real websocket through the
// graphql-transport-ws handshake to prove the swap kept working.
func TestBurgerBellSubscriptionOverWebsocket(t *testing.T) {
	db := testDB(t)

	srv := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: graph.NewResolver(db)}))
	srv.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
		Implementation: transport.CoderWebsocketImplementation{
			AcceptOptions: websocket.AcceptOptions{InsecureSkipVerify: true},
		},
	})
	srv.AddTransport(transport.POST{})

	httpSrv := httptest.NewServer(srv)
	defer httpSrv.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(httpSrv.URL, "http"), &websocket.DialOptions{
		Subprotocols: []string{"graphql-transport-ws"},
	})
	if err != nil {
		t.Fatalf("dialing the subscription websocket: %v", err)
	}
	defer conn.CloseNow()

	send := func(v map[string]any) {
		t.Helper()
		payload, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("encoding %v: %v", v, err)
		}
		if err := conn.Write(ctx, websocket.MessageText, payload); err != nil {
			t.Fatalf("writing %v: %v", v, err)
		}
	}
	recv := func() map[string]any {
		t.Helper()
		_, payload, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("reading from the websocket: %v", err)
		}
		var msg map[string]any
		if err := json.Unmarshal(payload, &msg); err != nil {
			t.Fatalf("decoding %s: %v", payload, err)
		}
		return msg
	}

	send(map[string]any{"type": "connection_init"})
	if ack := recv(); ack["type"] != "connection_ack" {
		t.Fatalf("got %v, want a connection_ack", ack)
	}

	send(map[string]any{
		"id":      "1",
		"type":    "subscribe",
		"payload": map[string]any{"query": `subscription { burgerBell { message timestamp } }`},
	})

	// Give the resolver a moment to register before ringing the bell.
	time.Sleep(200 * time.Millisecond)
	var ring struct {
		RingBurgerBell bool `json:"ringBurgerBell"`
	}
	// Must go through the same server: subscribers live on that resolver instance.
	client.New(srv).MustPost(`mutation { ringBurgerBell(message: "burgers are here") }`, &ring)

	msg := recv()
	if msg["type"] != "next" {
		t.Fatalf("got %v, want a next message carrying the event", msg)
	}
	event := msg["payload"].(map[string]any)["data"].(map[string]any)["burgerBell"].(map[string]any)
	if event["message"] != "burgers are here" {
		t.Errorf("event message = %v, want %q", event["message"], "burgers are here")
	}
}
