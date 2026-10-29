package nostr

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	n "fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip19"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/api"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/config"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/safeurl"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

var ErrIgnored = errors.New("ignored Nostr note")

type Service struct {
	wg           sync.WaitGroup
	Store        *store.Store
	Config       config.Config
	Gateway, Bot n.SecretKey
	Limit        api.Limiter
}

func New(s *store.Store, c config.Config) (*Service, error) {
	if len(c.GatewaySecret) != 64 || len(c.BotSecret) != 64 {
		return nil, errors.New("Nostr secrets must be 64-character hex")
	}
	g, err := n.SecretKeyFromHex(c.GatewaySecret)
	if err != nil {
		return nil, errors.New("invalid gateway secret")
	}
	b, err := n.SecretKeyFromHex(c.BotSecret)
	if err != nil {
		return nil, errors.New("invalid bot secret")
	}
	if g.Public() == b.Public() {
		return nil, errors.New("identities must differ")
	}
	return &Service{Store: s, Config: c, Gateway: g, Bot: b}, nil
}
func (s *Service) Npub() string { return nip19.EncodeNpub(s.Bot.Public()) }

var urls = regexp.MustCompile(`https?://[^\s<>"\x00-\x1f]+`)

func (s *Service) Validate(e n.Event) (string, error) {
	now := time.Now().Unix()
	if e.Kind != 1 || len(e.Content) > 8192 || len(e.Tags) > 100 || int64(e.CreatedAt) > now+300 || int64(e.CreatedAt) < now-7*86400 || !e.CheckID() || !e.VerifySignature() || e.PubKey == s.Bot.Public() || e.PubKey == s.Gateway.Public() {
		return "", errors.New("invalid event")
	}
	mention := false
	for _, t := range e.Tags {
		if len(t) > 1 && t[0] == "p" && t[1] == s.Bot.Public().Hex() {
			mention = true
		}
	}
	if !mention {
		return "", errors.New("missing mention")
	}
	candidates := urls.FindAllString(e.Content, 3)
	if len(candidates) != 1 {
		return "", errors.New("submit exactly one URL")
	}
	return safeurl.Normalize(strings.TrimRight(candidates[0], ".,);]"))
}
func (s *Service) Ingest(ctx context.Context, e n.Event, relay string) error {
	u, err := s.Validate(e)
	if err != nil {
		return ErrIgnored
	}
	var existing int
	if err = s.Store.DB.QueryRowContext(ctx, "SELECT count(*) FROM nostr_events WHERE id=?", e.ID.Hex()).Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		return s.observe(ctx, e.ID.Hex(), relay)
	}
	if !s.Limit.Allow("author:"+e.PubKey.Hex(), 20) {
		return ErrIgnored
	}
	_, err = s.Store.Accept(ctx, store.Intake{Origin: "nostr", URL: u, CanonicalURL: u, Author: e.PubKey.Hex(), EventID: e.ID.Hex(), EventRaw: store.JSON(e), Provenance: relay})
	if err != nil {
		return err
	}
	return s.observe(ctx, e.ID.Hex(), relay)
}
func (s *Service) Listen(ctx context.Context) {
	for _, relay := range s.Config.Relays {
		s.wg.Add(1)
		go func() { defer s.wg.Done(); s.listenRelay(ctx, relay) }()
	}
	// The persisted signed metadata event is reused on restart.
	tx, err := s.Store.DB.BeginTx(ctx, nil)
	if err == nil {
		defer tx.Rollback()
		if store.Enqueue(tx, "outbox", "relay_list", s.relayListKey(), "{}", time.Now().Unix()) == nil {
			if err = tx.Commit(); err != nil {
				slog.Error("relay metadata enqueue failed")
			}
		}
	}
}
func (s *Service) listenRelay(ctx context.Context, url string) {
	for ctx.Err() == nil {
		s.subscription(ctx, url)
		select {
		case <-ctx.Done():
			return
		case <-time.After(15 * time.Second):
		}
	}
}
func (s *Service) subscription(ctx context.Context, url string) {
	r := n.NewRelay(ctx, url, n.RelayOptions{NoticeHandler: func(_ *n.Relay, _ string) { slog.Debug("Nostr relay notice") }})
	defer r.Close()
	connectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	err := r.ConnectWithClient(connectCtx, safeurl.Client())
	cancel()
	if err != nil {
		slog.Warn("Nostr relay reconnect pending", "relay", url)
		return
	}
	since := time.Now().Add(-24 * time.Hour).Unix()
	var cursor int64
	if s.Store.DB.QueryRowContext(ctx, "SELECT cursor FROM harvest_state WHERE provider=?", "nostr:"+url).Scan(&cursor) == nil {
		since = cursor - 300
	}
	if since < time.Now().Add(-7*24*time.Hour).Unix() {
		since = time.Now().Add(-7 * 24 * time.Hour).Unix()
	}
	sub, err := r.Subscribe(ctx, n.Filter{Kinds: []n.Kind{1}, Tags: n.TagMap{"p": {s.Bot.Public().Hex()}}, Since: n.Timestamp(since)}, n.SubscriptionOptions{})
	if err != nil {
		return
	}
	defer sub.Unsub()
	// Cursor is wall-clock after EOSE, never a potentially future-dated event timestamp.
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	caughtUp := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-sub.Context.Done():
			return
		case _, ok := <-sub.EndOfStoredEvents:
			if ok {
				caughtUp = true
			}
		case e, ok := <-sub.Events:
			if !ok {
				return
			}
			if err = s.Ingest(ctx, e, url); err != nil && !errors.Is(err, ErrIgnored) {
				return
			}
		case <-ticker.C:
			if caughtUp {
				_, err = s.Store.DB.ExecContext(ctx, "INSERT INTO harvest_state VALUES(?,?,?) ON CONFLICT(provider) DO UPDATE SET cursor=excluded.cursor,updated_at=excluded.updated_at", "nostr:"+url, time.Now().Unix(), time.Now().Unix())
				if err != nil {
					return
				}
			}
		}
	}
}

// ReplyTags preserves the original NIP-10 root, including a parent that is itself a reply.
func ReplyTags(parent n.Event) n.Tags {
	root := parent.ID.Hex()
	hint := ""
	for _, t := range parent.Tags {
		if len(t) > 3 && t[0] == "e" && t[3] == "root" {
			root = t[1]
			hint = t[2]
			break
		}
	}
	// NIP-10's deprecated positional format: first e is the root.
	if root == parent.ID.Hex() {
		for _, t := range parent.Tags {
			if len(t) > 1 && t[0] == "e" && (len(t) < 4 || t[3] == "") {
				root = t[1]
				if len(t) > 2 {
					hint = t[2]
				}
				break
			}
		}
	}
	tags := n.Tags{{"e", root, hint, "root"}, {"p", parent.PubKey.Hex()}, {"t", "cannabis-enforcement-status"}}
	if root != parent.ID.Hex() {
		tags = append(tags, n.Tag{"e", parent.ID.Hex(), "", "reply"})
	}
	return tags
}
func (s *Service) build(ctx context.Context, j store.Job) (n.Event, error) {
	e := n.Event{CreatedAt: n.Timestamp(time.Now().Unix()), Kind: 1, Tags: n.Tags{}}
	key := s.Bot
	if j.Kind == "relay_list" {
		e.Kind = 10002
		for _, relay := range s.Config.Relays {
			e.Tags = append(e.Tags, n.Tag{"r", relay})
		}
	}
	if j.Kind == "gateway" {
		v, err := s.Store.Get(ctx, strings.SplitN(j.Ref, ":", 2)[0])
		if err != nil {
			return e, err
		}
		key = s.Gateway
		e.Content = "Cannabis enforcement news submitted: " + v.URL
		if v.Note != "" {
			e.Content += "\nPublic note: " + v.Note
		}
		e.Tags = n.Tags{{"r", v.URL}, {"t", "cannabis-enforcement"}, {"p", s.Bot.Public().Hex()}}
	}
	if j.Kind == "resolution" {
		v, err := s.Store.Get(ctx, strings.SplitN(j.Ref, ":", 2)[0])
		if err != nil {
			return e, err
		}
		if !store.Terminal(v.Status) {
			return e, errors.New("case unresolved")
		}
		if v.NostrEventID == "" {
			return e, errors.New("gateway note pending")
		}
		var raw string
		if err = s.Store.DB.QueryRowContext(ctx, "SELECT raw FROM nostr_events WHERE id=?", v.NostrEventID).Scan(&raw); err != nil {
			return e, err
		}
		var parent n.Event
		if json.Unmarshal([]byte(raw), &parent) != nil {
			return e, errors.New("invalid parent")
		}
		e.Tags = ReplyTags(parent)
		labels := map[string]string{"published": "Published", "duplicate": "Already covered", "not_relevant": "Outside scope", "routed_to_cfi": "Routed to CFI", "insufficient_evidence": "Could not verify", "needs_review": "Needs review", "failed": "Processing failed"}
		e.Content = labels[v.Status] + ": " + v.Explanation + "\n" + s.Config.PublicURL + "/submission/?id=" + v.ID
		if v.ReportSlug != "" && (v.Status == "published" || v.Status == "duplicate") {
			e.Content += "\n" + s.Config.PublicURL + "/report/" + v.ReportSlug + "/"
		}
	}
	if err := e.Sign(key); err != nil {
		return e, err
	}
	return e, nil
}
func (s *Service) Deliver(ctx context.Context, j store.Job) error {
	if j.Kind == "resolution" && (j.Payload == "{}" || j.Payload == "") {
		parts := strings.SplitN(j.Ref, ":", 2)
		v, err := s.Store.Get(ctx, parts[0])
		if err != nil {
			return err
		}
		if len(parts) == 2 && parts[1] != v.Status {
			return nil
		}
	}

	var e n.Event
	if j.Payload != "{}" && j.Payload != "" {
		if err := json.Unmarshal([]byte(j.Payload), &e); err != nil {
			return err
		}
	} else {
		var err error
		e, err = s.build(ctx, j)
		if err != nil {
			return err
		}
		tx, err := s.Store.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		raw := store.JSON(e)
		if _, err = tx.ExecContext(ctx, "INSERT INTO nostr_events VALUES(?,?,?,?,?) ON CONFLICT DO NOTHING", e.ID.Hex(), raw, "out", j.Kind, time.Now().Unix()); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, "UPDATE outbox SET payload=? WHERE id=? AND lease_token=?", raw, j.ID, j.Token)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return errors.New("lease lost")
		}
		if j.Kind == "gateway" {
			_, err = tx.ExecContext(ctx, "UPDATE submissions SET gateway_event_id=? WHERE id=?", e.ID.Hex(), strings.SplitN(j.Ref, ":", 2)[0])
		}
		if j.Kind == "resolution" {
			_, err = tx.ExecContext(ctx, "UPDATE submissions SET reply_event_id=? WHERE id=?", e.ID.Hex(), strings.SplitN(j.Ref, ":", 2)[0])
		}
		if err != nil {
			return err
		}
		for _, relay := range s.Config.Relays {
			if _, err = tx.ExecContext(ctx, "INSERT INTO nostr_delivery_targets VALUES(?,?) ON CONFLICT DO NOTHING", e.ID.Hex(), relay); err != nil {
				return err
			}
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	if !e.CheckID() || !e.VerifySignature() {
		return errors.New("invalid signed outbox event")
	}
	if j.Kind == "resolution" {
		for _, tag := range e.Tags {
			if len(tag) > 1 && tag[0] == "p" {
				if author, err := n.PubKeyFromHex(tag[1]); err == nil && author != s.Gateway.Public() {
					for _, relay := range s.readRelays(ctx, author) {
						if _, err = s.Store.DB.ExecContext(ctx, "INSERT INTO nostr_delivery_targets VALUES(?,?) ON CONFLICT DO NOTHING", e.ID.Hex(), relay); err != nil {
							return err
						}
					}
				}
				break
			}
		}
	}
	for _, relay := range s.Config.Relays {
		if _, err := s.Store.DB.ExecContext(ctx, "INSERT INTO nostr_delivery_targets VALUES(?,?) ON CONFLICT DO NOTHING", e.ID.Hex(), relay); err != nil {
			return err
		}
	}
	rows, err := s.Store.DB.QueryContext(ctx, "SELECT relay FROM nostr_delivery_targets WHERE event_id=? ORDER BY relay", e.ID.Hex())
	if err != nil {
		return err
	}
	var targets []string
	for rows.Next() {
		var relay string
		if err = rows.Scan(&relay); err != nil {
			rows.Close()
			return err
		}
		targets = append(targets, relay)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return errors.New("no configured delivery relays")
	}
	pending := []string{}
	for _, relay := range targets {
		var accepted int
		err := s.Store.DB.QueryRowContext(ctx, "SELECT accepted FROM relay_receipts WHERE event_id=? AND relay=?", e.ID.Hex(), relay).Scan(&accepted)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if accepted != 1 {
			pending = append(pending, relay)
		}
	}
	type delivery struct {
		relay string
		ack   int
	}
	results := make(chan delivery, len(pending))
	slots := make(chan struct{}, 4)
	for _, relay := range pending {
		go func() {
			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
				results <- delivery{relay, 0}
				return
			}
			defer func() { <-slots }()
			attempt, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			r := n.NewRelay(attempt, relay, n.RelayOptions{NoticeHandler: func(_ *n.Relay, _ string) {}})
			defer r.Close()
			err := r.ConnectWithClient(attempt, safeurl.Client())
			if err == nil {
				err = r.Publish(attempt, e)
			}
			ack := 0
			if err == nil {
				ack = 1
			}
			results <- delivery{relay, ack}
		}()
	}
	all := true
	for range pending {
		result := <-results
		if result.ack != 1 {
			all = false
		}
		if _, err = s.Store.DB.ExecContext(ctx, `INSERT INTO relay_receipts VALUES(?,?,?,1,?) ON CONFLICT(event_id,relay) DO UPDATE SET accepted=excluded.accepted,attempts=attempts+1,updated_at=excluded.updated_at`, e.ID.Hex(), result.relay, result.ack, time.Now().Unix()); err != nil {
			return err
		}
	}
	if !all {
		return errors.New("relay acknowledgment pending")
	}
	return nil
}

// NIP-65 recipient read relays supplement the configured delivery relays.
func (s *Service) readRelays(ctx context.Context, author n.PubKey) []string {
	var newest n.Event
	for _, relay := range s.Config.Relays[:min(2, len(s.Config.Relays))] {
		attempt, cancel := context.WithTimeout(ctx, 3*time.Second)
		r := n.NewRelay(attempt, relay, n.RelayOptions{NoticeHandler: func(_ *n.Relay, _ string) {}})
		if r.ConnectWithClient(attempt, safeurl.Client()) == nil {
			sub, err := r.Subscribe(attempt, n.Filter{Kinds: []n.Kind{10002}, Authors: []n.PubKey{author}, Limit: 1}, n.SubscriptionOptions{})
			if err == nil {
				select {
				case e := <-sub.Events:
					if len(e.Tags) <= 100 && len(e.Content) <= 8192 && e.Kind == 10002 && e.PubKey == author && e.CheckID() && e.VerifySignature() && e.CreatedAt <= n.Timestamp(time.Now().Add(5*time.Minute).Unix()) && e.CreatedAt > newest.CreatedAt {
						newest = e
					}
				case <-attempt.Done():
				case <-sub.EndOfStoredEvents:
				}
				sub.Unsub()
			}
		}
		r.Close()
		cancel()
	}
	targets := []string{}
	seen := map[string]bool{}
	for _, tag := range newest.Tags {
		if len(tag) < 2 || tag[0] != "r" || (len(tag) > 2 && tag[2] != "read") {
			continue
		}
		if len(tag[1]) > 4096 {
			continue
		}
		u, err := url.Parse(tag[1])
		if err != nil || u.Scheme != "wss" || u.User != nil || u.Host == "" || (u.Port() != "" && u.Port() != "443") || seen[u.String()] {
			continue
		}
		seen[u.String()] = true
		targets = append(targets, u.String())
		if len(targets) == 3 {
			break
		}
	}
	return targets
}

func (s *Service) relayListKey() string {
	sum := sha256.Sum256([]byte(strings.Join(s.Config.Relays, "\n")))
	return s.Bot.Public().Hex() + ":" + hex.EncodeToString(sum[:8])
}

func (s *Service) Wait() { s.wg.Wait() }

func (s *Service) observe(ctx context.Context, id, relay string) error {
	now := time.Now().Unix()
	_, err := s.Store.DB.ExecContext(ctx, "INSERT INTO nostr_observations VALUES(?,?,?,?) ON CONFLICT(event_id,relay) DO UPDATE SET last_seen=excluded.last_seen", id, relay, now, now)
	return err
}
