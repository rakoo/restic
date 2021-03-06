package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/restic/restic"
	"github.com/restic/restic/backend"
)

type CmdLs struct{}

func init() {
	_, err := parser.AddCommand("ls",
		"list files",
		"The ls command lists all files and directories in a snapshot",
		&CmdLs{})
	if err != nil {
		panic(err)
	}
}

func print_node(prefix string, n *restic.Node) string {
	switch n.Type {
	case "file":
		return fmt.Sprintf("%s %5d %5d %6d %s %s",
			n.Mode, n.UID, n.GID, n.Size, n.ModTime, filepath.Join(prefix, n.Name))
	case "dir":
		return fmt.Sprintf("%s %5d %5d %6d %s %s",
			n.Mode|os.ModeDir, n.UID, n.GID, n.Size, n.ModTime, filepath.Join(prefix, n.Name))
	case "symlink":
		return fmt.Sprintf("%s %5d %5d %6d %s %s -> %s",
			n.Mode|os.ModeSymlink, n.UID, n.GID, n.Size, n.ModTime, filepath.Join(prefix, n.Name), n.LinkTarget)
	default:
		return fmt.Sprintf("<Node(%s) %s>", n.Type, n.Name)
	}
}

func print_tree(prefix string, s restic.Server, blob restic.Blob) error {
	tree, err := restic.LoadTree(s, blob)
	if err != nil {
		return err
	}

	for _, entry := range tree.Nodes {
		fmt.Println(print_node(prefix, entry))

		if entry.Type == "dir" && entry.Subtree != nil {
			b, err := tree.Map.FindID(entry.Subtree)
			if err != nil {
				return err
			}

			err = print_tree(filepath.Join(prefix, entry.Name), s, b)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (cmd CmdLs) Usage() string {
	return "snapshot-ID [DIR]"
}

func (cmd CmdLs) Execute(args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("wrong number of arguments, Usage: %s", cmd.Usage())
	}

	s, err := OpenRepo()
	if err != nil {
		return err
	}

	name, err := backend.FindSnapshot(s, args[0])
	if err != nil {
		return err
	}

	id, err := backend.ParseID(name)
	if err != nil {
		return err
	}

	sn, err := restic.LoadSnapshot(s, id)
	if err != nil {
		return err
	}

	fmt.Printf("snapshot of %v at %s:\n", sn.Paths, sn.Time)

	return print_tree("", s, sn.Tree)
}
