package backup

import (
	// Register supported blob storage providers
	_ "github.com/kopia/kopia/repo/blob/filesystem"
	_ "github.com/kopia/kopia/repo/blob/s3"
)
