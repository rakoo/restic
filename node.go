package restic

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/juju/arrar"
	"github.com/restic/restic/backend"
)

type Node struct {
	Name       string       `json:"name"`
	Type       string       `json:"type"`
	Mode       os.FileMode  `json:"mode,omitempty"`
	ModTime    time.Time    `json:"mtime,omitempty"`
	AccessTime time.Time    `json:"atime,omitempty"`
	ChangeTime time.Time    `json:"ctime,omitempty"`
	UID        uint32       `json:"uid"`
	GID        uint32       `json:"gid"`
	User       string       `json:"user,omitempty"`
	Group      string       `json:"group,omitempty"`
	Inode      uint64       `json:"inode,omitempty"`
	Size       uint64       `json:"size,omitempty"`
	Links      uint64       `json:"links,omitempty"`
	LinkTarget string       `json:"linktarget,omitempty"`
	Device     uint64       `json:"device,omitempty"`
	Content    []backend.ID `json:"content"`
	Subtree    backend.ID   `json:"subtree,omitempty"`

	Error string `json:"error,omitempty"`

	tree *Tree

	path  string
	err   error
	blobs Blobs
}

func (n Node) String() string {
	switch n.Type {
	case "file":
		return fmt.Sprintf("%s %5d %5d %6d %s %s",
			n.Mode, n.UID, n.GID, n.Size, n.ModTime, n.Name)
	case "dir":
		return fmt.Sprintf("%s %5d %5d %6d %s %s",
			n.Mode|os.ModeDir, n.UID, n.GID, n.Size, n.ModTime, n.Name)
	}

	return fmt.Sprintf("<Node(%s) %s>", n.Type, n.Name)
}

func (node Node) Tree() *Tree {
	return node.tree
}

func NodeFromFileInfo(path string, fi os.FileInfo) (*Node, error) {
	node := &Node{
		path:    path,
		Name:    fi.Name(),
		Mode:    fi.Mode() & os.ModePerm,
		ModTime: fi.ModTime(),
	}

	node.Type = nodeTypeFromFileInfo(path, fi)
	if node.Type == "file" {
		node.Size = uint64(fi.Size())
	}

	err := node.fill_extra(path, fi)
	return node, err
}

func nodeTypeFromFileInfo(path string, fi os.FileInfo) string {
	switch fi.Mode() & (os.ModeType | os.ModeCharDevice) {
	case 0:
		return "file"
	case os.ModeDir:
		return "dir"
	case os.ModeSymlink:
		return "symlink"
	case os.ModeDevice | os.ModeCharDevice:
		return "chardev"
	case os.ModeDevice:
		return "dev"
	case os.ModeNamedPipe:
		return "fifo"
	case os.ModeSocket:
		return "socket"
	}

	return ""
}

func CreateNodeAt(node *Node, m *Map, s Server, path string) error {
	switch node.Type {
	case "dir":
		err := os.Mkdir(path, node.Mode)
		if err != nil {
			return arrar.Annotate(err, "Mkdir")
		}

		err = os.Lchown(path, int(node.UID), int(node.GID))
		if err != nil {
			return arrar.Annotate(err, "Lchown")
		}

		var utimes = []syscall.Timespec{
			syscall.NsecToTimespec(node.AccessTime.UnixNano()),
			syscall.NsecToTimespec(node.ModTime.UnixNano()),
		}
		err = syscall.UtimesNano(path, utimes)
		if err != nil {
			return arrar.Annotate(err, "Utimesnano")
		}
	case "file":
		// TODO: handle hard links
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
		defer f.Close()
		if err != nil {
			return arrar.Annotate(err, "OpenFile")
		}

		for _, blobid := range node.Content {
			blob, err := m.FindID(blobid)
			if err != nil {
				return arrar.Annotate(err, "Find Blob")
			}

			buf, err := s.Load(backend.Data, blob)
			if err != nil {
				return arrar.Annotate(err, "Load")
			}

			_, err = f.Write(buf)
			if err != nil {
				return arrar.Annotate(err, "Write")
			}
		}

		f.Close()

		err = os.Lchown(path, int(node.UID), int(node.GID))
		if err != nil {
			return arrar.Annotate(err, "Lchown")
		}

		var utimes = []syscall.Timespec{
			syscall.NsecToTimespec(node.AccessTime.UnixNano()),
			syscall.NsecToTimespec(node.ModTime.UnixNano()),
		}
		err = syscall.UtimesNano(path, utimes)
		if err != nil {
			return arrar.Annotate(err, "Utimesnano")
		}
	case "symlink":
		err := os.Symlink(node.LinkTarget, path)
		if err != nil {
			return arrar.Annotate(err, "Symlink")
		}

		err = os.Lchown(path, int(node.UID), int(node.GID))
		if err != nil {
			return arrar.Annotate(err, "Lchown")
		}

		// f, err := os.OpenFile(path, O_PATH|syscall.O_NOFOLLOW, 0600)
		// defer f.Close()
		// if err != nil {
		// 	return arrar.Annotate(err, "OpenFile")
		// }

		// TODO: Get Futimes() working on older Linux kernels (fails with 3.2.0)
		// var utimes = []syscall.Timeval{
		// 	syscall.NsecToTimeval(node.AccessTime.UnixNano()),
		// 	syscall.NsecToTimeval(node.ModTime.UnixNano()),
		// }
		// err = syscall.Futimes(int(f.Fd()), utimes)
		// if err != nil {
		// 	return arrar.Annotate(err, "Futimes")
		// }

		return nil
	case "dev":
		err := node.createDevAt(path)
		if err != nil {
			return arrar.Annotate(err, "Mknod")
		}
	case "chardev":
		err := node.createCharDevAt(path)
		if err != nil {
			return arrar.Annotate(err, "Mknod")
		}
	case "fifo":
		err := node.createFifoAt(path)
		if err != nil {
			return arrar.Annotate(err, "Mkfifo")
		}
	case "socket":
		// nothing to do, we do not restore sockets
		return nil
	default:
		return fmt.Errorf("filetype %q not implemented!\n", node.Type)
	}

	err := os.Chmod(path, node.Mode)
	if err != nil {
		return arrar.Annotate(err, "Chmod")
	}

	err = os.Chown(path, int(node.UID), int(node.GID))
	if err != nil {
		return arrar.Annotate(err, "Chown")
	}

	err = os.Chtimes(path, node.AccessTime, node.ModTime)
	if err != nil {
		return arrar.Annotate(err, "Chtimes")
	}

	return nil
}

func (node Node) SameContent(olderNode *Node) bool {
	// if this node has a type other than "file", treat as if content has changed
	if node.Type != "file" {
		return false
	}

	// if the name or type has changed, this is surely something different
	if node.Name != olderNode.Name || node.Type != olderNode.Type {
		return false
	}

	// if timestamps or inodes differ, content has changed
	if node.ModTime != olderNode.ModTime ||
		node.ChangeTime != olderNode.ChangeTime ||
		node.Inode != olderNode.Inode {
		return false
	}

	// otherwise the node is assumed to have the same content
	return true
}

func (node Node) MarshalJSON() ([]byte, error) {
	type nodeJSON Node
	nj := nodeJSON(node)
	name := strconv.Quote(node.Name)
	nj.Name = name[1 : len(name)-1]

	return json.Marshal(nj)
}

func (node *Node) UnmarshalJSON(data []byte) error {
	type nodeJSON Node
	var nj *nodeJSON = (*nodeJSON)(node)

	err := json.Unmarshal(data, nj)
	if err != nil {
		return err
	}

	nj.Name, err = strconv.Unquote(`"` + nj.Name + `"`)
	return err
}

func (node Node) Equals(other Node) bool {
	// TODO: add generatored code for this
	if node.Name != other.Name {
		return false
	}
	if node.Type != other.Type {
		return false
	}
	if node.Mode != other.Mode {
		return false
	}
	if node.ModTime != other.ModTime {
		return false
	}
	if node.AccessTime != other.AccessTime {
		return false
	}
	if node.ChangeTime != other.ChangeTime {
		return false
	}
	if node.UID != other.UID {
		return false
	}
	if node.GID != other.GID {
		return false
	}
	if node.User != other.User {
		return false
	}
	if node.Group != other.Group {
		return false
	}
	if node.Inode != other.Inode {
		return false
	}
	if node.Size != other.Size {
		return false
	}
	if node.Links != other.Links {
		return false
	}
	if node.LinkTarget != other.LinkTarget {
		return false
	}
	if node.Device != other.Device {
		return false
	}
	if node.Content != nil && other.Content == nil {
		return false
	} else if node.Content == nil && other.Content != nil {
		return false
	} else if node.Content != nil && other.Content != nil {
		if len(node.Content) != len(other.Content) {
			return false
		}

		for i := 0; i < len(node.Content); i++ {
			if !node.Content[i].Equal(other.Content[i]) {
				return false
			}
		}
	}

	if !node.Subtree.Equal(other.Subtree) {
		return false
	}

	if node.Error != other.Error {
		return false
	}

	return true
}
