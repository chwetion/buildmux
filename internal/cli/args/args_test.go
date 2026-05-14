package args

import (
	"reflect"
	"strings"
	"testing"

	"github.com/chwetion/buildmux/internal/output"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name      string
		in        []string
		platforms []string
		out       output.Spec
		rest      []string
		wantErr   string
	}{
		{
			name: "two platforms space form",
			in: []string{
				"--frontend", "dockerfile.v0",
				"--local", "context=.",
				"--opt", "platform=linux/amd64,linux/arm64",
				"--output", "type=image,name=foo:v1,push=true",
			},
			platforms: []string{"linux/amd64", "linux/arm64"},
			out:       output.Spec{Type: "image", Name: "foo:v1", Push: true},
			rest: []string{
				"--frontend", "dockerfile.v0",
				"--local", "context=.",
			},
		},
		{
			name: "equals form",
			in: []string{
				"--opt=platform=linux/amd64",
				"--output=type=image,name=foo:v1,push=true",
			},
			platforms: []string{"linux/amd64"},
			out:       output.Spec{Type: "image", Name: "foo:v1", Push: true},
			rest:      []string{},
		},
		{
			name: "repeated opt platform",
			in: []string{
				"--opt", "platform=linux/amd64",
				"--opt", "platform=linux/arm64",
				"--output", "type=image,name=foo:v1,push=true",
			},
			platforms: []string{"linux/amd64", "linux/arm64"},
			out:       output.Spec{Type: "image", Name: "foo:v1", Push: true},
			rest:      []string{},
		},
		{
			name: "non-platform opt preserved",
			in: []string{
				"--opt", "build-arg:FOO=bar",
				"--opt", "platform=linux/amd64",
				"--output", "type=image,name=foo:v1,push=true",
			},
			platforms: []string{"linux/amd64"},
			out:       output.Spec{Type: "image", Name: "foo:v1", Push: true},
			rest:      []string{"--opt", "build-arg:FOO=bar"},
		},
		{
			name: "missing platform",
			in: []string{
				"--output", "type=image,name=foo:v1,push=true",
			},
			wantErr: "--opt platform",
		},
		{
			name: "missing output",
			in: []string{
				"--opt", "platform=linux/amd64",
			},
			wantErr: "--output",
		},
		{
			name: "duplicate output",
			in: []string{
				"--opt", "platform=linux/amd64",
				"--output", "type=image,name=foo:v1,push=true",
				"--output", "type=image,name=bar:v1,push=true",
			},
			wantErr: "duplicate",
		},
		{
			name: "output type not image",
			in: []string{
				"--opt", "platform=linux/amd64",
				"--output", "type=oci,dest=/tmp/x",
			},
			wantErr: "type=image",
		},
		{
			name: "output push false",
			in: []string{
				"--opt", "platform=linux/amd64",
				"--output", "type=image,name=foo:v1,push=false",
			},
			wantErr: "push=true",
		},
		{
			name: "output missing name",
			in: []string{
				"--opt", "platform=linux/amd64",
				"--output", "type=image,push=true",
			},
			wantErr: "name=",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.in)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err=%v want substring %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got.Platforms, tc.platforms) {
				t.Errorf("platforms got %v want %v", got.Platforms, tc.platforms)
			}
			if !reflect.DeepEqual(got.Output, tc.out) {
				t.Errorf("output got %+v want %+v", got.Output, tc.out)
			}
			if !reflect.DeepEqual(got.Rest, tc.rest) {
				t.Errorf("rest got %v want %v", got.Rest, tc.rest)
			}
		})
	}
}
