package output

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    Spec
		wantErr bool
	}{
		{
			name: "image push true name",
			in:   "type=image,name=foo:v1,push=true",
			want: Spec{Type: "image", Name: "foo:v1", Push: true},
		},
		{
			name: "reorder still parses",
			in:   "name=foo:v1,push=true,type=image",
			want: Spec{Type: "image", Name: "foo:v1", Push: true},
		},
		{
			name: "extras preserved",
			in:   "type=image,name=foo:v1,push=true,registry.insecure=true",
			want: Spec{
				Type: "image", Name: "foo:v1", Push: true,
				Other: []KV{{K: "registry.insecure", V: "true"}},
			},
		},
		{
			name: "oci no push",
			in:   "type=oci,dest=/tmp/x",
			want: Spec{Type: "oci", Other: []KV{{K: "dest", V: "/tmp/x"}}},
		},
		{name: "bad push", in: "type=image,push=maybe", wantErr: true},
		{name: "missing equals", in: "type=image,push", wantErr: true},
		{name: "empty", in: "", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
		})
	}
}

func TestString(t *testing.T) {
	cases := []struct {
		name string
		in   Spec
		want string
	}{
		{
			name: "canonical order",
			in:   Spec{Type: "image", Name: "foo:v1", Push: true},
			want: "type=image,name=foo:v1,push=true",
		},
		{
			name: "extras appended in original order",
			in: Spec{
				Type: "image", Name: "foo:v1", Push: true,
				Other: []KV{{K: "registry.insecure", V: "true"}, {K: "compression", V: "zstd"}},
			},
			want: "type=image,name=foo:v1,push=true,registry.insecure=true,compression=zstd",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in.String()
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
