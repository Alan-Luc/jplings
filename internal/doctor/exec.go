package doctor

import (
	"net"
	"os/exec"
	"path/filepath"
	"time"
)

func execLookPath(name string) (string, error) { return exec.LookPath(name) }

func filepathGlob(pattern string) ([]string, error) { return filepath.Glob(pattern) }

// dial reports whether something accepts a TCP connection at addr, closing it
// immediately: the check is reachability, not a protocol handshake.
func dial(network, addr string, timeout time.Duration) error {
	conn, err := net.DialTimeout(network, addr, timeout)
	if err != nil {
		return err
	}
	return conn.Close()
}
