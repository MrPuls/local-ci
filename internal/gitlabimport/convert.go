// Package gitlabimport converts a .gitlab-ci.yml into a local-ci config.
// The formats are siblings, so most of a typical file maps one-to-one
// (stages, scripts, images, variables, services, artifacts, needs, retry,
// timeout, cache, extends, parallel:matrix). Whatever has no local-ci
// equivalent is dropped and reported as a note instead of failing the
// import — the goal is a runnable starting point plus an honest list of
// what didn't carry over.
package gitlabimport

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"
)

// supportedJobKeys lists job-level keys we convert; everything else becomes a note.
var supportedJobKeys = map[string]bool{
	"stage": true, "image": true, "script": true, "before_script": true,
	"variables": true, "services": true, "artifacts": true, "needs": true,
	"retry": true, "timeout": true, "cache": true, "extends": true,
	"parallel": true,
}

type Result struct {
	YAML  []byte
	Notes []string
}

type converter struct {
	notes []string
	// defaults from the top-level `default:` block / legacy top-level keys
	defaultImage        string
	defaultBeforeScript []string
	defaultServices     []*yaml.Node
}

func (c *converter) notef(format string, args ...any) {
	c.notes = append(c.notes, fmt.Sprintf(format, args...))
}

// Convert translates .gitlab-ci.yml content into local-ci YAML plus notes
// about everything that could not be carried over.
func Convert(data []byte) (*Result, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse .gitlab-ci.yml: %w", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("unexpected .gitlab-ci.yml structure: top level must be a mapping")
	}
	root := doc.Content[0]

	c := &converter{}
	out := &yaml.Node{Kind: yaml.MappingNode}
	out.HeadComment = "Imported from .gitlab-ci.yml by `local-ci import gitlab`.\nReview the conversion notes printed by the importer."

	// Pass 1: top-level settings and defaults.
	var stages []string
	var variables *yaml.Node
	type rawJob struct {
		name string
		node *yaml.Node
	}
	var jobs []rawJob

	for i := 0; i < len(root.Content); i += 2 {
		key, val := root.Content[i].Value, root.Content[i+1]
		switch key {
		case "stages":
			_ = val.Decode(&stages)
		case "variables":
			variables = c.convertVariables(val)
		case "default":
			c.readDefaults(val)
		case "image":
			c.defaultImage = imageName(val)
		case "before_script":
			c.defaultBeforeScript = scriptLines(val)
		case "services":
			c.defaultServices = c.convertServices(val, "default")
		case "after_script":
			c.notef("top-level after_script dropped: local-ci has no always-runs-after hook (consider per-job script lines or job_cleanup for host-side commands)")
		case "include":
			c.notef("include dropped: GitLab includes are not fetched; local-ci has its own file-based include: key if you split configs")
		case "workflow":
			c.notef("workflow rules dropped: local-ci runs are always started explicitly")
		default:
			if val.Kind == yaml.MappingNode {
				jobs = append(jobs, rawJob{name: key, node: val})
			} else {
				c.notef("top-level key %q dropped (not a job mapping)", key)
			}
		}
	}

	if len(stages) == 0 {
		stages = []string{"build", "test", "deploy"}
		c.notef("no stages defined; using GitLab's implicit [build, test, deploy]")
	}
	appendKV(out, "stages", encode(stages))
	if variables != nil && len(variables.Content) > 0 {
		appendKV(out, "variables", variables)
	}

	// Pass 2: jobs, in file order.
	for _, j := range jobs {
		jobNode := c.convertJob(j.name, j.node, stages)
		appendKV(out, j.name, jobNode)
	}

	buf, err := marshalWithIndent(out)
	if err != nil {
		return nil, err
	}
	sort.Strings(c.notes)
	return &Result{YAML: buf, Notes: dedupe(c.notes)}, nil
}

func (c *converter) readDefaults(val *yaml.Node) {
	for i := 0; i+1 < len(val.Content); i += 2 {
		k, v := val.Content[i].Value, val.Content[i+1]
		switch k {
		case "image":
			c.defaultImage = imageName(v)
		case "before_script":
			c.defaultBeforeScript = scriptLines(v)
		case "services":
			c.defaultServices = c.convertServices(v, "default")
		default:
			c.notef("default.%s dropped: not supported by local-ci", k)
		}
	}
}

func (c *converter) convertJob(name string, node *yaml.Node, stages []string) *yaml.Node {
	out := &yaml.Node{Kind: yaml.MappingNode}
	isTemplate := strings.HasPrefix(name, ".")

	get := func(key string) *yaml.Node {
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				return node.Content[i+1]
			}
		}
		return nil
	}

	// stage — required by local-ci (except for templates).
	stage := ""
	if n := get("stage"); n != nil {
		stage = n.Value
	}
	hasExtends := get("extends") != nil
	if stage == "" && !isTemplate && !hasExtends {
		stage = "test" // GitLab's implicit default
		if !contains(stages, stage) {
			stage = stages[0]
		}
		c.notef("job %q had no stage; assigned %q", name, stage)
	}
	if stage != "" {
		appendKV(out, "stage", scalar(stage))
	}

	// image — fall back to the default block, then flag.
	img := ""
	if n := get("image"); n != nil {
		img = imageName(n)
	}
	if img == "" {
		img = c.defaultImage
	}
	if img != "" {
		appendKV(out, "image", scalar(img))
	} else if !isTemplate && !hasExtends {
		todo := scalar("alpine:3.21")
		todo.LineComment = "TODO: no image in .gitlab-ci.yml — set the right one"
		appendKV(out, "image", todo)
		c.notef("job %q has no image (GitLab runners have a configured default); placeholder inserted", name)
	}

	if n := get("extends"); n != nil {
		appendKV(out, "extends", n)
	}
	if n := get("variables"); n != nil {
		if v := c.convertVariables(n); len(v.Content) > 0 {
			appendKV(out, "variables", v)
		}
	}

	// services: job-level wins; otherwise inherit the default block's.
	if n := get("services"); n != nil {
		if svcs := c.convertServices(n, name); len(svcs) > 0 {
			appendKV(out, "services", seq(svcs))
		}
	} else if len(c.defaultServices) > 0 {
		appendKV(out, "services", seq(c.defaultServices))
	}

	if n := get("needs"); n != nil {
		if needs := c.convertNeeds(n, name); len(needs) > 0 {
			appendKV(out, "needs", encode(needs))
		}
	}
	if n := get("retry"); n != nil {
		if r, ok := retryCount(n); ok {
			appendKV(out, "retry", encode(r))
		} else {
			c.notef("job %q: retry value not convertible; dropped", name)
		}
	}
	if n := get("timeout"); n != nil {
		if d, err := parseGitlabDuration(n.Value); err == nil {
			appendKV(out, "timeout", scalar(d.String()))
		} else {
			c.notef("job %q: timeout %q not parseable; dropped", name, n.Value)
		}
	}
	if n := get("cache"); n != nil {
		c.convertCache(n, name, out)
	}
	if n := get("artifacts"); n != nil {
		c.convertArtifacts(n, name, out)
	}
	if n := get("parallel"); n != nil {
		c.convertParallel(n, name, out)
	}

	// script (with before_script folded in, GitLab-style).
	before := scriptLines(get("before_script"))
	if before == nil {
		before = c.defaultBeforeScript
	}
	script := append(append([]string{}, before...), scriptLines(get("script"))...)
	if len(script) > 0 {
		appendKV(out, "script", encode(script))
	} else if !isTemplate && !hasExtends {
		c.notef("job %q has no script (likely a trigger/bridge job); placeholder inserted", name)
		appendKV(out, "script", encode([]string{"echo TODO: no script in source job"}))
	}

	// Everything else: dropped with a note.
	for i := 0; i+1 < len(node.Content); i += 2 {
		k := node.Content[i].Value
		if !supportedJobKeys[k] {
			c.notef("job %q: %q dropped (%s)", name, k, dropReason(k))
		}
	}
	return out
}

func dropReason(key string) string {
	switch key {
	case "rules", "only", "except", "when":
		return "conditional execution is not supported; local-ci runs what you ask for"
	case "after_script":
		return "no always-runs-after hook; move into script or host-side job_cleanup"
	case "dependencies":
		return "artifacts flow to all later jobs automatically"
	case "environment", "release", "pages", "coverage", "dast_configuration":
		return "GitLab platform feature"
	case "tags", "interruptible", "resource_group", "allow_failure", "inherit", "secrets", "trigger", "id_tokens":
		return "GitLab runner/platform setting with no local equivalent"
	default:
		return "no local-ci equivalent"
	}
}

// --- field converters -------------------------------------------------------

func (c *converter) convertVariables(node *yaml.Node) *yaml.Node {
	out := &yaml.Node{Kind: yaml.MappingNode}
	for i := 0; i+1 < len(node.Content); i += 2 {
		k, v := node.Content[i].Value, node.Content[i+1]
		switch v.Kind {
		case yaml.ScalarNode:
			appendKV(out, k, scalar(v.Value))
		case yaml.MappingNode: // {value: x, description: ...}
			for j := 0; j+1 < len(v.Content); j += 2 {
				if v.Content[j].Value == "value" {
					appendKV(out, k, scalar(v.Content[j+1].Value))
				}
			}
		}
	}
	return out
}

func (c *converter) convertServices(node *yaml.Node, owner string) []*yaml.Node {
	var out []*yaml.Node
	for _, entry := range node.Content {
		switch entry.Kind {
		case yaml.ScalarNode:
			out = append(out, scalar(entry.Value))
		case yaml.MappingNode:
			svc := &yaml.Node{Kind: yaml.MappingNode}
			for i := 0; i+1 < len(entry.Content); i += 2 {
				k, v := entry.Content[i].Value, entry.Content[i+1]
				switch k {
				case "name":
					appendKV(svc, "image", scalar(v.Value))
				case "alias":
					appendKV(svc, "alias", scalar(v.Value))
				case "variables":
					appendKV(svc, "variables", c.convertVariables(v))
				default:
					c.notef("job %q: service key %q dropped (no local-ci equivalent)", owner, k)
				}
			}
			out = append(out, svc)
		}
	}
	return out
}

func (c *converter) convertNeeds(node *yaml.Node, owner string) []string {
	var out []string
	for _, entry := range node.Content {
		switch entry.Kind {
		case yaml.ScalarNode:
			out = append(out, entry.Value)
		case yaml.MappingNode:
			for i := 0; i+1 < len(entry.Content); i += 2 {
				k, v := entry.Content[i].Value, entry.Content[i+1]
				switch k {
				case "job":
					out = append(out, v.Value)
				case "optional", "artifacts":
					// optional deps and artifact toggles have no equivalent
					c.notef("job %q: needs.%s dropped (dependencies are always required; artifacts always flow)", owner, k)
				case "pipeline", "project", "ref":
					c.notef("job %q: cross-pipeline needs dropped", owner)
				}
			}
		}
	}
	if node.Kind == yaml.SequenceNode && len(node.Content) == 0 {
		c.notef("job %q: 'needs: []' (start immediately) has no equivalent; job keeps stage order", owner)
	}
	return out
}

func retryCount(node *yaml.Node) (int, bool) {
	if node.Kind == yaml.ScalarNode {
		var n int
		if err := node.Decode(&n); err == nil {
			return n, true
		}
		return 0, false
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == "max" {
			var n int
			if err := node.Content[i+1].Decode(&n); err == nil {
				return n, true
			}
		}
	}
	return 0, false
}

func (c *converter) convertCache(node *yaml.Node, owner string, out *yaml.Node) {
	if node.Kind == yaml.SequenceNode { // multiple caches: take the first
		if len(node.Content) == 0 {
			return
		}
		c.notef("job %q: multiple caches defined; only the first was imported", owner)
		node = node.Content[0]
	}
	cache := &yaml.Node{Kind: yaml.MappingNode}
	var key string
	var paths []string
	for i := 0; i+1 < len(node.Content); i += 2 {
		k, v := node.Content[i].Value, node.Content[i+1]
		switch k {
		case "key":
			if v.Kind == yaml.ScalarNode {
				key = v.Value
			} else {
				c.notef("job %q: cache key from files is not supported; using a static key", owner)
				key = sanitizeKey(owner) + "-imported"
			}
		case "paths":
			_ = v.Decode(&paths)
		default:
			c.notef("job %q: cache.%s dropped", owner, k)
		}
	}
	if key == "" {
		key = sanitizeKey(owner) + "-cache"
	}
	if len(paths) == 0 {
		return
	}
	appendKV(cache, "key", scalar(key))
	appendKV(cache, "paths", encode(paths))
	appendKV(out, "cache", cache)
}

func (c *converter) convertArtifacts(node *yaml.Node, owner string, out *yaml.Node) {
	var paths []string
	for i := 0; i+1 < len(node.Content); i += 2 {
		k, v := node.Content[i].Value, node.Content[i+1]
		switch k {
		case "paths":
			_ = v.Decode(&paths)
		case "expire_in", "when", "name", "untracked", "exclude", "public":
			// storage/visibility knobs are irrelevant locally — quiet drop
		case "reports":
			c.notef("job %q: artifacts.reports dropped (GitLab platform feature)", owner)
		default:
			c.notef("job %q: artifacts.%s dropped", owner, k)
		}
	}
	if len(paths) == 0 {
		return
	}
	a := &yaml.Node{Kind: yaml.MappingNode}
	appendKV(a, "paths", encode(paths))
	appendKV(out, "artifacts", a)
}

func (c *converter) convertParallel(node *yaml.Node, owner string, out *yaml.Node) {
	if node.Kind == yaml.ScalarNode {
		c.notef("job %q: 'parallel: %s' (N copies) dropped; local-ci parallelism is matrix- or mode-based", owner, node.Value)
		return
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == "matrix" {
			appendKV(out, "matrix", node.Content[i+1])
			return
		}
	}
}

// parseGitlabDuration accepts both Go-style ("1h30m") and GitLab's human
// format ("3 hours 30 minutes", "30 minutes").
func parseGitlabDuration(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(strings.ReplaceAll(s, " ", "")); err == nil {
		return d, nil
	}
	norm := strings.ToLower(s)
	// Longest unit names first, or "minutes" would be mangled by "min".
	for _, p := range []struct{ unit, repl string }{
		{"minutes", "m"}, {"minute", "m"}, {"mins", "m"}, {"min", "m"},
		{"seconds", "s"}, {"second", "s"}, {"secs", "s"}, {"sec", "s"},
		{"hours", "h"}, {"hour", "h"}, {"hrs", "h"}, {"hr", "h"},
		{"days", "d"}, {"day", "d"},
	} {
		norm = strings.ReplaceAll(norm, p.unit, p.repl)
	}
	norm = strings.ReplaceAll(norm, " ", "")
	if i := strings.Index(norm, "d"); i > 0 { // Go has no "d" unit
		var days int
		if _, err := fmt.Sscanf(norm[:i], "%d", &days); err == nil {
			rest, _ := time.ParseDuration(strings.TrimPrefix(norm[i+1:], ""))
			if norm[i+1:] == "" {
				rest = 0
			}
			return time.Duration(days)*24*time.Hour + rest, nil
		}
	}
	return time.ParseDuration(norm)
}

// --- yaml.Node helpers -------------------------------------------------------

func scalar(s string) *yaml.Node {
	n := &yaml.Node{}
	_ = n.Encode(s)
	return n
}

func encode(v any) *yaml.Node {
	n := &yaml.Node{}
	_ = n.Encode(v)
	return n
}

func seq(items []*yaml.Node) *yaml.Node {
	return &yaml.Node{Kind: yaml.SequenceNode, Content: items}
}

func appendKV(m *yaml.Node, key string, value *yaml.Node) {
	m.Content = append(m.Content, scalar(key), value)
}

func imageName(node *yaml.Node) string {
	if node.Kind == yaml.ScalarNode {
		return node.Value
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == "name" {
			return node.Content[i+1].Value
		}
	}
	return ""
}

// scriptLines flattens a script value: a string, a list of strings, or
// GitLab's nested lists (YAML anchors often produce them).
func scriptLines(node *yaml.Node) []string {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.ScalarNode {
		return []string{node.Value}
	}
	var out []string
	for _, entry := range node.Content {
		out = append(out, scriptLines(entry)...)
	}
	return out
}

func sanitizeKey(s string) string {
	return strings.Trim(strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, s), "-.")
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func dedupe(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := in[:0]
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func marshalWithIndent(node *yaml.Node) ([]byte, error) {
	var sb strings.Builder
	enc := yaml.NewEncoder(&sb)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return []byte(sb.String()), nil
}
