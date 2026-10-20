package cli

import (
	"reflect"
	"testing"
)

func TestFlagsParseLongShortAndEquals(t *testing.T) {
	fs := newFlagSet("test")
	var engine, dir string
	var stories, quiet bool
	var limit int
	var extra []string
	fs.String(&engine, "engine", "e")
	fs.String(&dir, "dir", "d")
	fs.BoolPair(&stories, "stories", "no-stories")
	fs.Bool(&quiet, "quiet", "Q")
	fs.Int(&limit, "limit", "n")
	fs.List(&extra, "engine-arg")

	args := []string{
		"--engine=mock", "-d", "/tmp/x", "--stories", "-n", "5",
		"https://a", "--engine-arg", "--no-part", "--quiet", "https://b",
	}
	if err := fs.parse(args); err != nil {
		t.Fatal(err)
	}
	if engine != "mock" || dir != "/tmp/x" || !stories || !quiet || limit != 5 {
		t.Fatalf("значения разобраны неверно: %q %q %v %v %d", engine, dir, stories, quiet, limit)
	}
	if !fs.Changed("engine") || !fs.Changed("no-stories") == false {
		// no-stories не задавали — Changed должен быть false
		if fs.Changed("no-stories") {
			t.Fatal("no-stories не должен быть отмечен")
		}
	}
	want := []string{"https://a", "https://b"}
	if !reflect.DeepEqual(fs.Positionals(), want) {
		t.Fatalf("позиционные аргументы: %v", fs.Positionals())
	}
	if !reflect.DeepEqual(extra, []string{"--no-part"}) {
		t.Fatalf("engine-arg: %v", extra)
	}
}

func TestBoolPairSwitchesOff(t *testing.T) {
	fs := newFlagSet("test")
	var stories bool
	fs.BoolPair(&stories, "stories", "no-stories")
	if err := fs.parse([]string{"--stories", "--no-stories"}); err != nil {
		t.Fatal(err)
	}
	if stories {
		t.Fatal("--no-stories должен выключать")
	}
}

func TestUnknownFlagFails(t *testing.T) {
	fs := newFlagSet("test")
	var dir string
	fs.String(&dir, "dir", "d")
	if err := fs.parse([]string{"--nope"}); err == nil {
		t.Fatal("неизвестный флаг должен давать ошибку")
	}
}

func TestMissingValueFails(t *testing.T) {
	fs := newFlagSet("test")
	var dir string
	fs.String(&dir, "dir", "d")
	if err := fs.parse([]string{"--dir"}); err == nil {
		t.Fatal("отсутствие значения должно давать ошибку")
	}
}

func TestDoubleDashStopsParsing(t *testing.T) {
	fs := newFlagSet("test")
	var dir string
	fs.String(&dir, "dir", "d")
	if err := fs.parse([]string{"--", "--dir", "-x"}); err != nil {
		t.Fatal(err)
	}
	if dir != "" {
		t.Fatal("после -- флаги не разбираются")
	}
	if len(fs.Positionals()) != 2 {
		t.Fatalf("позиционные аргументы: %v", fs.Positionals())
	}
}

func TestIntFlagRejectsGarbage(t *testing.T) {
	fs := newFlagSet("test")
	var limit int
	fs.Int(&limit, "limit", "n")
	if err := fs.parse([]string{"--limit", "abc"}); err == nil {
		t.Fatal("нечисловое значение должно давать ошибку")
	}
}
