package java

import "testing"

func TestOneLTSPerVendorDedupesAndSorts(t *testing.T) {
	pkgs := []pkg{
		{Distribution: "zulu", JavaVersion: "25.0.3+9", TermOfSupport: "lts"},
		{Distribution: "zulu", JavaVersion: "25.0.3", TermOfSupport: "lts"}, // musl duplicate, dropped
		{Distribution: "temurin", JavaVersion: "25.0.3+9", TermOfSupport: "lts"},
		{Distribution: "corretto", JavaVersion: "25.0.3", TermOfSupport: "lts"},
	}

	got := oneLTSPerVendor(pkgs)
	if len(got) != 3 {
		t.Fatalf("got %d versions, want 3 (one per vendor): %+v", len(got), got)
	}

	want := []string{"corretto-25.0.3", "temurin-25.0.3+9", "zulu-25.0.3+9"}
	for i, w := range want {
		if got[i].Raw != w {
			t.Fatalf("got[%d] = %q, want %q (order: %+v)", i, got[i].Raw, w, got)
		}
		if !got[i].LTS {
			t.Fatalf("got[%d] = %+v, want LTS", i, got[i])
		}
	}
}

func TestOneLTSPerVendorEmpty(t *testing.T) {
	if got := oneLTSPerVendor(nil); len(got) != 0 {
		t.Fatalf("got %+v, want empty", got)
	}
}
