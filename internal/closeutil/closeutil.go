// Package closeutil holds a small helper for the common main() pattern of
// closing a resource on shutdown and logging, rather than propagating, any
// error it returns.
package closeutil

import "log"

// LogClose calls close and logs its error, if any, prefixed with name. It is
// meant for deferred cleanup in main(), where by that point there is no
// caller left to return an error to.
func LogClose(name string, close func() error) {
	if err := close(); err != nil {
		log.Printf("close %s: %v", name, err)
	}
}
