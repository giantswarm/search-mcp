package version

import (
	"io"
	"os"

	"github.com/spf13/cobra"
)

const (
	name             = "version"
	shortDescription = "Print version information"
	longDescription  = "Print version information including git commit, Go version, and architecture"
)

type Config struct {
	Stdout io.Writer
	Stderr io.Writer
}

func New(config Config) (*cobra.Command, error) {
	if config.Stdout == nil {
		config.Stdout = os.Stdout
	}
	if config.Stderr == nil {
		config.Stderr = os.Stderr
	}

	r := &runner{
		stdout: config.Stdout,
		stderr: config.Stderr,
	}

	c := &cobra.Command{
		Use:   name,
		Short: shortDescription,
		Long:  longDescription,
		RunE:  r.Run,
	}

	return c, nil
}
