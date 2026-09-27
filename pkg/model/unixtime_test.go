package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestUnixTimeRoundTrip(t *testing.T) {
	want := UnixTime(time.Unix(1790000000, 0))

	v, err := want.Value()
	if err != nil {
		t.Fatal(err)
	}

	var got UnixTime
	err = got.Scan(v)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Time().Equal(want.Time()) {
		t.Fatalf("scan: got %v, want %v", got.Time(), want.Time())
	}

	b, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var fromJSON UnixTime
	err = json.Unmarshal(b, &fromJSON)
	if err != nil {
		t.Fatal(err)
	}
	if !fromJSON.Time().Equal(want.Time()) {
		t.Fatalf("json: got %v, want %v", fromJSON.Time(), want.Time())
	}

	err = got.Scan("nope")
	if err == nil {
		t.Fatal("expected error scanning string")
	}
}
