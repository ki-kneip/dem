package java

import "testing"

func TestGroupLTSByVendorTopNMajorsNewestFirst(t *testing.T) {
	pkgs := []pkg{
		{Distribution: "corretto", MajorVersion: 25, JavaVersion: "25.0.3"},
		{Distribution: "corretto", MajorVersion: 21, JavaVersion: "21.0.11"},
		{Distribution: "corretto", MajorVersion: 17, JavaVersion: "17.0.19"},
		{Distribution: "corretto", MajorVersion: 11, JavaVersion: "11.0.31"},
	}

	got := groupLTSByVendor(pkgs, 3)
	want := []string{"corretto-25.0.3", "corretto-21.0.11", "corretto-17.0.19"}
	if len(got) != len(want) {
		t.Fatalf("got %d versions, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Raw != w || got[i].Group != "corretto" || !got[i].LTS {
			t.Fatalf("got[%d] = %+v, want Raw=%q Group=corretto LTS=true", i, got[i], w)
		}
	}
}

func TestGroupLTSByVendorDedupesLibcVariants(t *testing.T) {
	pkgs := []pkg{
		{Distribution: "zulu", MajorVersion: 25, JavaVersion: "25.0.3+9"}, // glibc
		{Distribution: "zulu", MajorVersion: 25, JavaVersion: "25.0.3"},   // musl duplicate of the same major, dropped
	}

	got := groupLTSByVendor(pkgs, 3)
	if len(got) != 1 {
		t.Fatalf("got %d versions, want 1 (deduped by vendor+major): %+v", len(got), got)
	}
	if got[0].Raw != "zulu-25.0.3+9" {
		t.Fatalf("got %q, want the first build seen (zulu-25.0.3+9)", got[0].Raw)
	}
}

func TestGroupLTSByVendorSortsVendorsAlphabetically(t *testing.T) {
	pkgs := []pkg{
		{Distribution: "zulu", MajorVersion: 25, JavaVersion: "25.0.3"},
		{Distribution: "corretto", MajorVersion: 25, JavaVersion: "25.0.3"},
	}
	got := groupLTSByVendor(pkgs, 3)
	if len(got) != 2 || got[0].Group != "corretto" || got[1].Group != "zulu" {
		t.Fatalf("got %+v, want corretto before zulu", got)
	}
}

func TestGroupLTSByVendorFewerThanLimit(t *testing.T) {
	pkgs := []pkg{{Distribution: "eliya", MajorVersion: 25, JavaVersion: "25.0.3"}}
	got := groupLTSByVendor(pkgs, 3)
	if len(got) != 1 {
		t.Fatalf("got %d versions, want 1 (vendor has only one LTS major): %+v", len(got), got)
	}
}

func TestGroupLTSByVendorEmpty(t *testing.T) {
	if got := groupLTSByVendor(nil, 3); len(got) != 0 {
		t.Fatalf("got %+v, want empty", got)
	}
}
