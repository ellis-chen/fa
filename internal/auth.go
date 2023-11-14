package internal

var (

	// IdentityKey ...
	IdentityKey = "id"

	// CloudStorageKey ...
	CloudStorageKey = "cloud_storage"

	// UnkownPrincipal default principal, no auth verified
	UnkownPrincipal = UserPrincipal{UserID: -1, TenantID: 0, UserName: "unknown", LinuxUID: -1, LinuxGID: -1}

	// BackupProviderKey ...
	BackupProviderKey = "backup_provider"

	// BackupFullProviderKey ...
	BackupFullProviderKey = "backup_full_provider"
)

// UserPrincipal ...
type UserPrincipal struct {
	TenantID int64
	UserID   int
	UserName string
	LinuxUID int
	LinuxGID int
}
