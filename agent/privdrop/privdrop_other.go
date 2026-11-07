//go:build !linux

package privdrop

import "github.com/sirupsen/logrus"

// DropPrivileges is a no-op on platforms other than Linux. The Orthrus agent
// is only deployed as a Linux container; this stub keeps the package building
// for developer machines on other operating systems.
func DropPrivileges(_ *logrus.Logger) error {
	return nil
}
