// Package output parses and serializes the comma-separated KV string
// accepted by buildctl's --output flag (e.g. "type=image,name=foo:v1,push=true").
package output

import (
	"errors"
	"fmt"
	"strings"
)

type KV struct {
	K, V string
}

type Spec struct {
	Type  string
	Name  string
	Push  bool
	Other []KV
}

func Parse(s string) (Spec, error) {
	var spec Spec
	if strings.TrimSpace(s) == "" {
		return spec, errors.New("--output value is empty")
	}
	for _, p := range strings.Split(s, ",") {
		eq := strings.IndexByte(p, '=')
		if eq < 0 {
			return spec, fmt.Errorf("--output: expected k=v in %q", p)
		}
		k := strings.TrimSpace(p[:eq])
		v := p[eq+1:]
		switch k {
		case "type":
			spec.Type = v
		case "name":
			spec.Name = v
		case "push":
			switch v {
			case "true":
				spec.Push = true
			case "false":
				spec.Push = false
			default:
				return spec, fmt.Errorf("--output: push must be true or false, got %q", v)
			}
		default:
			spec.Other = append(spec.Other, KV{K: k, V: v})
		}
	}
	return spec, nil
}

func (s Spec) String() string {
	var b strings.Builder
	first := true
	write := func(k, v string) {
		if !first {
			b.WriteByte(',')
		}
		first = false
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(v)
	}
	if s.Type != "" {
		write("type", s.Type)
	}
	if s.Name != "" {
		write("name", s.Name)
	}
	push := "false"
	if s.Push {
		push = "true"
	}
	write("push", push)
	for _, kv := range s.Other {
		write(kv.K, kv.V)
	}
	return b.String()
}
