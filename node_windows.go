package restic

import "os"

func (node *Node) fill_extra(path string, fi os.FileInfo) error {
	return nil
}

func (node *Node) createDevAt(path string) error {
	return nil
}

func (node *Node) createCharDevAt(path string) error {
	return nil
}

func (node *Node) createFifoAt(path string) error {
	return nil
}
