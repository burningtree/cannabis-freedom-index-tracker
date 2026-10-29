package store

import (
	"context"
	"testing"
	"time"
)

// A validated draft's commit (publish) is taken before research, and the publish lane takes
// nothing else.
func TestClaimPutsPublishFirst(t *testing.T) {
	s := openTest(t)
	ctx := context.Background()
	tx, _ := s.DB.Begin()
	now := time.Now().Unix()
	Enqueue(tx, "jobs", "research", "case-a", "{}", now-60)
	Enqueue(tx, "jobs", "publish", "case-b", "{}", now)
	tx.Commit()
	j, err := s.Claim(ctx, "jobs", time.Minute)
	if err != nil || j.Kind != "publish" {
		t.Fatal(j, err)
	}
	if _, err = s.ClaimKind(ctx, "jobs", "publish", time.Minute); err == nil {
		t.Fatal("publish lane took research")
	}
	if j, err = s.Claim(ctx, "jobs", time.Minute); err != nil || j.Kind != "research" {
		t.Fatal(j, err)
	}
}
