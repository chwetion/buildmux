package config

import "testing"

func TestRenderTag(t *testing.T) {
	cases := []struct {
		name     string
		template string
		platform string
		image    string
		want     string
		wantErr  bool
	}{
		{
			name:     "name plus arch",
			template: "{{.Name}}-{{.Arch}}",
			platform: "linux/amd64",
			image:    "docker.io/me/app:v1",
			want:     "docker.io/me/app:v1-amd64",
		},
		{
			name:     "repo and tag separated",
			template: "{{.Repo}}:{{.Tag}}-{{.OS}}-{{.Arch}}",
			platform: "linux/amd64",
			image:    "docker.io/me/app:v1",
			want:     "docker.io/me/app:v1-linux-amd64",
		},
		{
			name:     "variant included",
			template: "{{.Repo}}:{{.Tag}}-{{.Arch}}{{.Variant}}",
			platform: "linux/arm/v7",
			image:    "me/app:v1",
			want:     "me/app:v1-armv7",
		},
		{
			name:     "image without tag uses latest",
			template: "{{.Repo}}:{{.Tag}}-{{.Arch}}",
			platform: "linux/amd64",
			image:    "me/app",
			want:     "me/app:latest-amd64",
		},
		{
			name:     "bad platform",
			template: "{{.Name}}-{{.Arch}}",
			platform: "garbage",
			image:    "me/app:v1",
			wantErr:  true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := Platform{Tag: tc.template}
			got, err := p.RenderTag(tc.platform, tc.image)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestSplitPlatform(t *testing.T) {
	cases := []struct {
		in            string
		os, arch, var_ string
		wantErr       bool
	}{
		{in: "linux/amd64", os: "linux", arch: "amd64"},
		{in: "linux/arm64", os: "linux", arch: "arm64"},
		{in: "linux/arm/v7", os: "linux", arch: "arm", var_: "v7"},
		{in: "windows/amd64", os: "windows", arch: "amd64"},
		{in: "linux", wantErr: true},
		{in: "", wantErr: true},
		{in: "a/b/c/d", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			pp, err := SplitPlatform(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if pp.OS != tc.os || pp.Arch != tc.arch || pp.Variant != tc.var_ {
				t.Fatalf("got %+v want os=%s arch=%s var=%s", pp, tc.os, tc.arch, tc.var_)
			}
		})
	}
}
