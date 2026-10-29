package nostr

import (
	"context"
	"encoding/json"
	n "fiatjaf.com/nostr"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/config"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
	"path/filepath"
	"testing"
	"time"
)

func testService(t *testing.T) *Service {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "nostr.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.DB.Close() })
	v, err := New(s, config.Config{BotSecret: n.Generate().Hex(), GatewaySecret: n.Generate().Hex(), Relays: []string{"wss://relay.example"}})
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func event(s *Service) n.Event {
	e := n.Event{Kind: 1, CreatedAt: n.Timestamp(time.Now().Unix()), Content: "Please investigate https://news.example/raid", Tags: n.Tags{{"p", s.Bot.Public().Hex()}}}
	e.Sign(n.Generate())
	return e
}
func TestVerifyMentionsAndReplay(t *testing.T) {
	s := testService(t)
	e := event(s)
	for range 2 {
		if err := s.Ingest(context.Background(), e, "wss://relay.example"); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	s.Store.DB.QueryRow("SELECT count(*) FROM submissions").Scan(&count)
	if count != 1 {
		t.Fatal(count)
	}
	bad := e
	bad.Content += "tampered"
	if _, err := s.Validate(bad); err == nil {
		t.Fatal("bad signature")
	}
	bad = event(s)
	bad.Tags = nil
	bad.Sign(n.Generate())
	if _, err := s.Validate(bad); err == nil {
		t.Fatal("missing p accepted")
	}
	bad = event(s)
	bad.Sign(s.Bot)
	if _, err := s.Validate(bad); err == nil {
		t.Fatal("bot loop")
	}
}
func TestThreadRootAndReply(t *testing.T) {
	s := testService(t)
	e := event(s)
	root := n.Generate().Public().Hex()
	e.Tags = append(e.Tags, n.Tag{"e", root, "wss://relay.example", "root"})
	e.Sign(n.Generate())
	tags := ReplyTags(e)
	if tags[0][1] != root || tags[0][3] != "root" || tags[len(tags)-1][1] != e.ID.Hex() || tags[len(tags)-1][3] != "reply" || tags[1][1] != e.PubKey.Hex() {
		t.Fatal(tags)
	}
	e = event(s)
	if len(ReplyTags(e)) != 3 {
		t.Fatal("root submission needs one root marker")
	}
}
func TestOutboxKeepsSignedEventOnOutage(t *testing.T) {
	s := testService(t)
	s.Config.Relays = []string{"ws://127.0.0.1:1"}
	id, err := s.Store.Accept(context.Background(), store.Intake{Origin: "web", URL: "https://news.example/a", CanonicalURL: "https://news.example/a", Gateway: true})
	if err != nil {
		t.Fatal(err)
	}
	j, err := s.Store.Claim(context.Background(), "outbox", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Deliver(context.Background(), j); err == nil {
		t.Fatal("relay outage not surfaced")
	}
	var payload string
	s.Store.DB.QueryRow("SELECT payload FROM outbox WHERE id=?", j.ID).Scan(&payload)
	var e n.Event
	if json.Unmarshal([]byte(payload), &e) != nil || !e.VerifySignature() {
		t.Fatal("signed event not persisted")
	}
	j.Payload = payload
	if err = s.Deliver(context.Background(), j); err == nil {
		t.Fatal("outage")
	}
	var second string
	s.Store.DB.QueryRow("SELECT payload FROM outbox WHERE id=?", j.ID).Scan(&second)
	if second != payload {
		t.Fatal("retry resigned event")
	}
	v, _ := s.Store.Get(context.Background(), id)
	if v.NostrEventID != e.ID.Hex() {
		t.Fatal("gateway reference missing")
	}
}
