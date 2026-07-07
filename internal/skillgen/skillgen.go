// Package skillgen generates the agent-facing skills/ tree (an index "gw2"
// skill, a "gw2-shared" conventions skill, and one "gw2-<group>" skill per
// top-level command group) from the live cobra command tree. Deriving the
// tree straight from the commands themselves, rather than hand-writing it,
// keeps the generated docs from drifting out of sync with the actual CLI
// surface as groups/leaves/flags are added.
package skillgen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// flagInfo is one local (non-inherited) flag on a leaf command.
type flagInfo struct {
	name  string // e.g. "ids"
	typ   string // pflag Value.Type(), e.g. "string", "bool", "int"
	usage string
}

// leaf is one data-fetching command discovered while walking a group's
// subtree: a command with a RunE. A "dual-role" command (one that both
// fetches data AND has subcommands, e.g. achievements, guild, account) is
// recorded as a leaf in its own right, in addition to its children being
// walked separately.
type leaf struct {
	path  string // full command path, e.g. "gw2 data mounts skins"
	short string
	flags []flagInfo
}

// group is one top-level gw2 command (data, account, character, ...).
type group struct {
	name   string
	short  string
	leaves []leaf
}

// Generate walks root's command tree and writes outDir/gw2/SKILL.md (the
// index), outDir/gw2-shared/SKILL.md (shared conventions), and one
// outDir/gw2-<group>/SKILL.md per top-level group -- every direct child of
// root except cobra's built-in "help"/"completion" commands and
// "generate-skills" itself. Output is fully deterministic: groups and their
// leaves are sorted alphabetically by path, so re-running Generate over an
// unchanged command tree produces byte-identical files.
func Generate(root *cobra.Command, version, outDir string) error {
	groups := collectGroups(root)

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	if err := writeSkill(outDir, "gw2", indexBody(version, groups)); err != nil {
		return err
	}
	if err := writeSkill(outDir, "gw2-shared", sharedBody(version)); err != nil {
		return err
	}
	for _, g := range groups {
		if err := writeSkill(outDir, "gw2-"+g.name, groupBody(g)); err != nil {
			return err
		}
	}
	return nil
}

// collectGroups treats each direct child of root (other than the cobra
// built-ins and generate-skills) as one top-level group, and recursively
// collects its leaf commands.
func collectGroups(root *cobra.Command) []group {
	var groups []group
	for _, c := range root.Commands() {
		switch c.Name() {
		case "help", "completion", "generate-skills":
			continue
		}
		g := group{name: c.Name(), short: c.Short}
		collectLeaves(c, "gw2 "+c.Name(), &g.leaves)
		sort.Slice(g.leaves, func(i, j int) bool { return g.leaves[i].path < g.leaves[j].path })
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].name < groups[j].name })
	return groups
}

// collectLeaves records cmd itself when it has a RunE (whether or not it
// also has children -- a "dual-role" command, e.g. achievements/guild's own
// bare invocation, is both), then recurses into every child regardless,
// since a plain grouping command (no RunE, e.g. data.go/account.go's
// addToTree-created intermediate groups) exists only to hold further
// leaves.
func collectLeaves(cmd *cobra.Command, path string, out *[]leaf) {
	if cmd.RunE != nil {
		*out = append(*out, leaf{path: path, short: cmd.Short, flags: localFlags(cmd)})
	}
	for _, child := range cmd.Commands() {
		collectLeaves(child, path+" "+child.Name(), out)
	}
}

// localFlags returns cmd's own (non-inherited) flags, sorted by name for
// determinism regardless of pflag's internal iteration/sort settings.
func localFlags(cmd *cobra.Command) []flagInfo {
	var flags []flagInfo
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		flags = append(flags, flagInfo{name: f.Name, typ: f.Value.Type(), usage: f.Usage})
	})
	sort.Slice(flags, func(i, j int) bool { return flags[i].name < flags[j].name })
	return flags
}

// writeSkill creates outDir/<name>/SKILL.md containing body (which already
// includes its own YAML frontmatter).
func writeSkill(outDir, name, body string) error {
	dir := filepath.Join(outDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644)
}

// yamlString returns s as a safely-quoted YAML double-quoted scalar: wrapped
// in double quotes, with embedded backslashes and double quotes escaped.
// These two escapes are sufficient to make any single-line text (including
// text containing a ": " sequence, which breaks an unquoted YAML plain
// scalar) a valid YAML double-quoted scalar. Every frontmatter
// `description:` value goes through this helper -- descriptions are derived
// from command `Short` text, which is free-form and not guaranteed to avoid
// YAML-special sequences.
func yamlString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// indexDescription is the fixed description text for gw2/SKILL.md, given
// verbatim by the design brief.
const indexDescription = "Guild Wars 2 API CLI — read-only client for accounts, trading post, WvW, PvP, guilds, and game data. Index of gw2-* skills."

// indexFrontmatter builds the fixed frontmatter for gw2/SKILL.md, given
// verbatim by the design brief (description safely quoted via yamlString).
func indexFrontmatter() string {
	return fmt.Sprintf("---\nname: gw2\ndescription: %s\n---\n", yamlString(indexDescription))
}

// indexBody builds the gw2/SKILL.md content: the fixed frontmatter above, a
// one-paragraph overview noting the CLI version, a bullet list of every
// gw2-<group> skill with its one-line description, and a pointer to
// gw2-shared.
func indexBody(version string, groups []group) string {
	var b strings.Builder
	b.WriteString(indexFrontmatter())
	fmt.Fprintf(&b, "\n# gw2 (v%s)\n\n", version)
	b.WriteString("Guild Wars 2 API CLI: a read-only command-line client for the official GW2\n")
	b.WriteString("API, covering account assets and unlocks, characters, the trading post,\n")
	b.WriteString("World vs World, PvP, guilds, and the game's static reference data. Each\n")
	b.WriteString("gw2-<group> skill below documents one top-level command group.\n\n")
	b.WriteString("## Command groups\n\n")
	for _, g := range groups {
		fmt.Fprintf(&b, "- `gw2-%s` — %s\n", g.name, g.short)
	}
	b.WriteString("\nSee `gw2-shared` for authentication setup, global flags, output modes, and\n")
	b.WriteString("exit codes shared by every command above.\n")
	return b.String()
}

// sharedDescription is the fixed description text for gw2-shared/SKILL.md,
// given verbatim by the design brief. It contains a ": " sequence, which is
// exactly why yamlString's quoting matters: emitted unquoted, this text
// breaks YAML parsing.
const sharedDescription = "Shared gw2 CLI conventions: auth, global flags, output modes, exit codes."

// sharedFrontmatter builds the fixed frontmatter for gw2-shared/SKILL.md,
// given verbatim by the design brief (description safely quoted via
// yamlString).
func sharedFrontmatter() string {
	return fmt.Sprintf("---\nname: gw2-shared\ndescription: %s\n---\n", yamlString(sharedDescription))
}

// sharedBody builds the gw2-shared/SKILL.md content: static conventions
// text (verified against internal/commands/app.go, internal/auth/store.go,
// and internal/api/errors.go so the flag names, config path, and exit codes
// below stay accurate) plus the current CLI version.
func sharedBody(version string) string {
	var b strings.Builder
	b.WriteString(sharedFrontmatter())
	fmt.Fprintf(&b, "\n# gw2-shared (v%s)\n\n", version)
	b.WriteString("Conventions shared by every gw2 command.\n\n")

	b.WriteString("## Global flags\n\n")
	b.WriteString("Available on every command:\n\n")
	b.WriteString("- `--profile <name>` — credential profile to use (default: whichever profile\n")
	b.WriteString("  was most recently set via `gw2 auth set`)\n")
	b.WriteString("- `--lang en|es|de|fr|zh` — response language for localized data (default: en)\n")
	b.WriteString("- `--raw` — print the API's raw JSON response, unmodified\n")
	b.WriteString("- `--json` — print the response as pretty-indented JSON\n\n")
	b.WriteString("With neither `--raw` nor `--json`, output is the concise, human-readable form.\n\n")

	b.WriteString("## Authentication\n\n")
	b.WriteString("The GW2 API is read-only through this CLI: no gw2 command mutates account or\n")
	b.WriteString("game state. Endpoints scoped to an account or guild (account, character,\n")
	b.WriteString("commerce transactions/delivery, pvp standings/stats, token, and the guild\n")
	b.WriteString("leader/officer subresources) need an API key with the right scope; everything\n")
	b.WriteString("else works anonymously.\n\n")
	b.WriteString("Generate a key at https://account.arena.net/applications, then store it:\n\n")
	b.WriteString("    gw2 auth set <profile> --key <APIKEY>\n\n")
	b.WriteString("Keys are encrypted (AES-256-GCM) and stored at `~/.config/gw2/config.toml`;\n")
	b.WriteString("`gw2 auth list` never prints the key value itself. Pass `--profile <name>` to\n")
	b.WriteString("select a stored key other than the default.\n\n")

	b.WriteString("## Output modes\n\n")
	b.WriteString("- concise (default) — a short, human-readable summary of the response\n")
	b.WriteString("- `--raw` — the API's JSON response, unmodified\n")
	b.WriteString("- `--json` — the same data, pretty-printed\n\n")

	b.WriteString("## Exit codes\n\n")
	b.WriteString("- `0` — success\n")
	b.WriteString("- `3` — authentication/permission error (API returned 403)\n")
	b.WriteString("- `4` — not found (API returned 404)\n")
	b.WriteString("- `5` — rate-limited (API returned 429)\n")
	b.WriteString("- `1` — any other error\n\n")

	b.WriteString("## Listing vs. fetching\n\n")
	b.WriteString("Endpoints that follow the \"enumerate ids, fetch by ids\" shape (most of\n")
	b.WriteString("gw2-data, gw2-account, gw2-wvw, gw2-pvp, and similar) print only the id list\n")
	b.WriteString("when called bare -- they never auto-dump every entry. Pass `--ids\n")
	b.WriteString("<comma-separated ids>` to fetch specific entries, or `--all` to fetch every\n")
	b.WriteString("entry (an explicit opt-in, since some of these lists are large).\n")

	return b.String()
}

// groupBody builds one gw2-<group>/SKILL.md: frontmatter naming the group
// and using its Short as the description, then a list of the group's leaf
// commands (full path, local flags, and Short).
func groupBody(g group) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nname: gw2-%s\ndescription: %s\n---\n\n", g.name, yamlString(g.short))
	fmt.Fprintf(&b, "# gw2-%s\n\n", g.name)
	fmt.Fprintf(&b, "Commands under `gw2 %s`.\n\n## Commands\n\n", g.name)
	for _, l := range g.leaves {
		flagsSuffix := ""
		if len(l.flags) > 0 {
			flagsSuffix = " [flags]"
		}
		fmt.Fprintf(&b, "- `%s%s` — %s\n", l.path, flagsSuffix, l.short)
		for _, f := range l.flags {
			placeholder := ""
			if f.typ != "bool" {
				placeholder = " <" + f.typ + ">"
			}
			fmt.Fprintf(&b, "  - `--%s%s` — %s\n", f.name, placeholder, f.usage)
		}
	}
	b.WriteString("\nSee `gw2-shared` for auth setup and global flags.\n")
	return b.String()
}
