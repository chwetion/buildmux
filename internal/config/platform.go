package config

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

type PlatformParts struct {
	OS      string
	Arch    string
	Variant string
}

func SplitPlatform(s string) (PlatformParts, error) {
	parts := strings.Split(s, "/")
	switch len(parts) {
	case 2:
		if parts[0] == "" || parts[1] == "" {
			return PlatformParts{}, fmt.Errorf("invalid platform %q", s)
		}
		return PlatformParts{OS: parts[0], Arch: parts[1]}, nil
	case 3:
		if parts[0] == "" || parts[1] == "" || parts[2] == "" {
			return PlatformParts{}, fmt.Errorf("invalid platform %q", s)
		}
		return PlatformParts{OS: parts[0], Arch: parts[1], Variant: parts[2]}, nil
	default:
		return PlatformParts{}, fmt.Errorf("invalid platform %q: expected os/arch[/variant]", s)
	}
}

type tagContext struct {
	Name    string
	Repo    string
	Tag     string
	OS      string
	Arch    string
	Variant string
}

func (p Platform) RenderTag(platform, image string) (string, error) {
	pp, err := SplitPlatform(platform)
	if err != nil {
		return "", err
	}
	repo, tag := splitImageRef(image)
	ctx := tagContext{
		Name:    image,
		Repo:    repo,
		Tag:     tag,
		OS:      pp.OS,
		Arch:    pp.Arch,
		Variant: pp.Variant,
	}
	tpl, err := template.New("tag").Parse(p.Tag)
	if err != nil {
		return "", fmt.Errorf("parse tag template: %w", err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("execute tag template: %w", err)
	}
	return buf.String(), nil
}

func splitImageRef(image string) (repo, tag string) {
	// Find the last ':' that is after the last '/'. Anything before is repo,
	// after is tag. If no such ':' exists, repo is the whole thing, tag = "latest".
	lastSlash := strings.LastIndexByte(image, '/')
	lastColon := strings.LastIndexByte(image, ':')
	if lastColon > lastSlash {
		return image[:lastColon], image[lastColon+1:]
	}
	return image, "latest"
}
