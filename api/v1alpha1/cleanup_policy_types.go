package v1alpha1

const (
	// Don't touch resources on cleanup
	CleanupPolicySkip = string("Skip")
	// Delete associated resources on cleanup
	CleanupPolicyDelete = string("Delete")
	// Revoke associated grant on cleanup
	CleanupPolicyRevoke = string("Revoke")
)
