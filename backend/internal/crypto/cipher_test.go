package crypto

import "testing"

// Compile-time check: the rotation service is the production cipher.
func TestRotationService_SatisfiesVersionedCipher(t *testing.T) {
	var _ VersionedCipher = (*RotationService)(nil)
}
