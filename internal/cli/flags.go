package cli

import (
	"fmt"
	"strconv"
	"strings"
)

type flagKind int

const (
	kindString flagKind = iota
	kindBool
	kindInt
	kindList
)

type flagSpec struct {
	names []string
	kind  flagKind
	set   func(string) error
}

// flagSet — небольшой парсер аргументов, понимающий --флаг, --флаг=знач,
// короткие -ф, повторяемые флаги и позиционные аргументы вперемешку.
type flagSet struct {
	name    string
	specs   map[string]*flagSpec
	args    []string
	changed map[string]bool
}

func newFlagSet(name string) *flagSet {
	return &flagSet{name: name, specs: map[string]*flagSpec{}, changed: map[string]bool{}}
}

func (fs *flagSet) add(kind flagKind, set func(string) error, names ...string) {
	spec := &flagSpec{names: names, kind: kind, set: set}
	for _, name := range names {
		fs.specs[name] = spec
	}
}

func (fs *flagSet) String(target *string, names ...string) {
	fs.add(kindString, func(v string) error { *target = v; return nil }, names...)
}

func (fs *flagSet) Int(target *int, names ...string) {
	fs.add(kindInt, func(v string) error {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("ожидалось число, получено %q", v)
		}
		*target = n
		return nil
	}, names...)
}

func (fs *flagSet) Bool(target *bool, names ...string) {
	fs.add(kindBool, func(v string) error {
		b, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("ожидалось true/false, получено %q", v)
		}
		*target = b
		return nil
	}, names...)
}

// BoolPair регистрирует пару флагов: включающий (--stories) и выключающий (--no-stories).
func (fs *flagSet) BoolPair(target *bool, on string, off string) {
	fs.Bool(target, on)
	fs.add(kindBool, func(string) error { *target = false; return nil }, off)
}

func (fs *flagSet) List(target *[]string, names ...string) {
	fs.add(kindList, func(v string) error { *target = append(*target, v); return nil }, names...)
}

func (fs *flagSet) Changed(name string) bool { return fs.changed[name] }

func (fs *flagSet) Positionals() []string { return fs.args }

func (fs *flagSet) markChanged(spec *flagSpec) {
	for _, name := range spec.names {
		fs.changed[name] = true
	}
}

func (fs *flagSet) parse(input []string) error {
	for i := 0; i < len(input); i++ {
		arg := input[i]
		if arg == "--" {
			fs.args = append(fs.args, input[i+1:]...)
			break
		}
		if len(arg) > 1 && strings.HasPrefix(arg, "-") {
			name, value, hasValue := splitFlag(arg)
			spec, ok := fs.specs[name]
			if !ok {
				return fmt.Errorf("неизвестный флаг: %s", arg)
			}
			if spec.kind == kindBool {
				if !hasValue {
					value = "true"
				}
			} else if !hasValue {
				i++
				if i >= len(input) {
					return fmt.Errorf("для флага %s нужно значение", arg)
				}
				value = input[i]
			}
			if err := spec.set(value); err != nil {
				return fmt.Errorf("флаг %s: %w", arg, err)
			}
			fs.markChanged(spec)
			continue
		}
		fs.args = append(fs.args, arg)
	}
	return nil
}

func splitFlag(arg string) (name, value string, hasValue bool) {
	trimmed := strings.TrimLeft(arg, "-")
	if index := strings.Index(trimmed, "="); index >= 0 {
		return trimmed[:index], trimmed[index+1:], true
	}
	return trimmed, "", false
}
