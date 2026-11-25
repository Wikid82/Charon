//go:build !unix

package services

import "io/fs"

// openNoFollowFlags is empty where the platform has no no-follow open flag. Go plugins are not loaded on Windows.
const openNoFollowFlags = 0

// ownedBy always reports true where file ownership is not checked. Go plugins are not loaded on Windows.
func ownedBy(fs.FileInfo, int) bool { return true }
