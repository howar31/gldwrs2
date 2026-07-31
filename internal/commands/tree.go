package commands

import "github.com/spf13/cobra"

// addToTree walks segments from root, creating intermediate group commands
// as needed (reusing one already created for a sibling resource that shares
// a segment prefix), and attaches the command returned by buildLeaf at the
// final segment. Shared by data.go's catalog registry and account.go's
// account registry, both of which register a flat slice of resources under
// nested command groups derived from each resource's segment path.
//
// A segment that is itself both a leaf (has a RunE) and a parent (has
// children) is fine: cobra dispatches to the matching child by name and
// falls back to the parent's own RunE otherwise. Callers that rely on this
// (e.g. a resource that's both an endpoint and a parent of narrower
// resources) must register the parent-and-leaf resource before its children
// so the tree builder finds the existing leaf command to descend into,
// rather than creating a duplicate plain group command.
func addToTree(root *cobra.Command, segments []string, buildLeaf func(use string) *cobra.Command) {
	cur := root
	for i, seg := range segments {
		if i == len(segments)-1 {
			cur.AddCommand(buildLeaf(seg))
			return
		}
		child := findSubcommand(cur, seg)
		if child == nil {
			child = &cobra.Command{
				Use:   seg,
				Short: "Subcommands for " + seg,
			}
			cur.AddCommand(child)
		}
		cur = child
	}
}

func findSubcommand(parent *cobra.Command, use string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == use {
			return c
		}
	}
	return nil
}
