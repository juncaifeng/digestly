package store

import (
	"testing"
)

func TestFeedMaxItemsRoundTrip(t *testing.T) {
	st := openTest(t)
	fid := addFeed(t, st, "cap")
	if err := st.SetFeedMaxItems(fid, 50); err != nil {
		t.Fatal(err)
	}
	feeds, _ := st.ListFeeds()
	if feeds[0].MaxItems != 50 {
		t.Fatalf("MaxItems = %d, want 50", feeds[0].MaxItems)
	}
}
