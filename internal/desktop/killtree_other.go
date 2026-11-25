//go:build !windows

package desktop

// killTree is a no-op outside Windows: the plain Kill of the process is enough there.
func killTree(int) {}
