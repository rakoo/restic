package backend_test

import (
	"flag"
	"io/ioutil"
	"os"
	"testing"

	"github.com/restic/restic/backend/sftp"
)

var sftpPath = flag.String("test.sftppath", "", "sftp binary path (default: empty)")

func setupSFTPBackend(t *testing.T) *sftp.SFTP {
	tempdir, err := ioutil.TempDir("", "restic-test-")
	ok(t, err)

	b, err := sftp.Create(tempdir, *sftpPath)
	ok(t, err)

	t.Logf("created sftp backend locally at %s", tempdir)

	return b
}

func teardownSFTPBackend(t *testing.T, b *sftp.SFTP) {
	if !*testCleanup {
		t.Logf("leaving backend at %s\n", b.Location())
		return
	}

	err := os.RemoveAll(b.Location())
	ok(t, err)
}

func TestSFTPBackend(t *testing.T) {
	if *sftpPath == "" {
		t.Skipf("sftppath not set, skipping TestSFTPBackend")
	}

	s := setupSFTPBackend(t)
	defer teardownSFTPBackend(t, s)

	testBackend(s, t)
}
