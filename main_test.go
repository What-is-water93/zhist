package main

import (
	"bytes"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

func searchFixture() []Row {
	return []Row{
		{Entry: Entry{T: 1, D: "/a", C: "git status"}},
		{Entry: Entry{T: 2, D: "/b", C: "git push"}},
		{Entry: Entry{T: 3, D: "/a", C: "ls -la"}},
		{Entry: Entry{T: 4, D: "/b", C: "git status"}},
		{Entry: Entry{T: 5, D: "/a", C: "go test ./..."}},
	}
}

func TestSearchRowsPrefixNewestFirstDedup(t *testing.T) {
	got := searchRows(searchFixture(), "git", "", 0)
	// "git status" ran twice; only the newest run survives.
	want := []string{"git status", "git push"}
	if !slices.Equal(got, want) {
		t.Errorf("searchRows(git) = %q, want %q", got, want)
	}
}

func TestSearchRowsDirFilter(t *testing.T) {
	got := searchRows(searchFixture(), "git", "/a", 0)
	want := []string{"git status"}
	if !slices.Equal(got, want) {
		t.Errorf("searchRows(git, /a) = %q, want %q", got, want)
	}
}

func TestSearchRowsLimit(t *testing.T) {
	got := searchRows(searchFixture(), "g", "", 1)
	want := []string{"go test ./..."}
	if !slices.Equal(got, want) {
		t.Errorf("searchRows(g, limit 1) = %q, want %q", got, want)
	}
}

func TestSearchRowsEmptyPrefixMatchesAll(t *testing.T) {
	got := searchRows(searchFixture(), "", "", 0)
	want := []string{"go test ./...", "git status", "ls -la", "git push"}
	if !slices.Equal(got, want) {
		t.Errorf("searchRows(\"\") = %q, want %q", got, want)
	}
}

func TestSearchRowsNoMatch(t *testing.T) {
	if got := searchRows(searchFixture(), "cargo", "", 0); len(got) != 0 {
		t.Errorf("searchRows(cargo) = %q, want empty", got)
	}
}

func TestSearchRowsUnsortedInput(t *testing.T) {
	rows := searchFixture()
	slices.Reverse(rows)
	got := searchRows(rows, "git", "", 0)
	want := []string{"git status", "git push"}
	if !slices.Equal(got, want) {
		t.Errorf("searchRows(git, reversed input) = %q, want %q", got, want)
	}
}

func TestAbbrevHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory available")
	}
	cases := []struct {
		dir  string
		want string
	}{
		{home, "~"},
		{home + "/code/zhist", "~/code/zhist"},
		{"/var/empty", "/var/empty"},
		{"", ""},
	}
	for _, c := range cases {
		if got := abbrevHome(c.dir); got != c.want {
			t.Errorf("abbrevHome(%q) = %q, want %q", c.dir, got, c.want)
		}
	}
}

func TestTruncatePath(t *testing.T) {
	cases := []struct {
		path string
		max  int
		want string
	}{
		{"~/code/zhist", 0, "~/code/zhist"},
		{"~/code/zhist", 100, "~/code/zhist"},
		{"~/code/zhist", 12, "~/code/zhist"},
		{"~/code/zhist", 8, "…e/zhist"},
		{"~/code/zhist", 1, "t"},
		{"héllo/wörld", 7, "…/wörld"},
	}
	for _, c := range cases {
		if got := truncatePath(c.path, c.max); got != c.want {
			t.Errorf("truncatePath(%q, %d) = %q, want %q", c.path, c.max, got, c.want)
		}
	}
}

func TestFmtDur(t *testing.T) {
	cases := []struct {
		ms   int64
		want string
	}{
		{0, ""},
		{-5, ""},
		{1, "1ms"},
		{999, "999ms"},
		{1000, "1.0s"},
		{1250, "1.2s"},
		{59_999, "59.9s"},
		{60_000, "1m00s"},
		{83_000, "1m23s"},
		{3_599_000, "59m59s"},
		{3_600_000, "1h00m"},
		{9_000_000, "2h30m"},
		{90_000_000, "25h00m"},
	}
	for _, c := range cases {
		if got := fmtDur(c.ms); got != c.want {
			t.Errorf("fmtDur(%d) = %q, want %q", c.ms, got, c.want)
		}
	}
}

func TestPickerIgnoresPathWhenMatching(t *testing.T) {
	fzf, err := exec.LookPath("fzf")
	if err != nil {
		t.Skip("fzf not installed")
	}
	rows := []Row{
		{ID: "a", Entry: Entry{T: 1, D: "/home/test", C: "ls"}},
		{ID: "b", Entry: Entry{T: 2, D: "/home/other", C: "test -f x"}},
	}
	var list bytes.Buffer
	writeList(&list, rows, "", 0, 80, 100)

	args := []string{"--ansi", "--filter=test"}
	args = append(args, strings.Fields(strings.ReplaceAll(fzfFieldFlags, "'", ""))...)
	cmd := exec.Command(fzf, args...)
	cmd.Env = append(os.Environ(), "FZF_DEFAULT_OPTS=", "FZF_DEFAULT_OPTS_FILE=")
	cmd.Stdin = &list
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("fzf: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "b\t") {
		t.Errorf("query %q matched %q, want only the command containing it (ID b)", "test", lines)
	}
}
