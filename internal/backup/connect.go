package backup

import (
	"context"
	"os"
	"path/filepath"

	"github.com/kopia/kopia/repo"
	"github.com/pkg/errors"
)

// OpenRepository connects to the kopia repository based on the config stored in the config file
// NOTE: This assumes that `kopia repository connect` has been already run on the machine
// OR the above Connect function has been used to connect to the repository server
func OpenRepository(ctx context.Context, configFile, password string) (repo.RepositoryWriter, error) {
	repoConfig := repositoryConfigFileName(configFile)
	if _, err := os.Stat(repoConfig); os.IsNotExist(err) {
		return nil, errors.New("Failed find kopia configuration file")
	}

	r, err := repo.Open(ctx, repoConfig, password, &repo.Options{
		DisableInternalLog: true,
	})
	if os.IsNotExist(err) {
		return nil, errors.New("Failed to find kopia repository, use `kopia repository create` or kopia repository connect` if already created")
	}

	if err != nil {
		return nil, errors.Wrap(err, "Failed to open kopia repository")
	}

	return r.(repo.RepositoryWriter), nil
}

func repositoryConfigFileName(configFile string) string {
	if configFile != "" {
		return configFile
	}

	return filepath.Join(os.Getenv("HOME"), ".config", "kopia", "repository.config")
}
