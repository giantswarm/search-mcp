package version

import (
	"fmt"
	"io"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/giantswarm/search-mcp/pkg/project"
)

type runner struct {
	stdout io.Writer
	stderr io.Writer
}

func (r *runner) Run(cmd *cobra.Command, args []string) error {
	_, _ = fmt.Fprintf(r.stdout, "Version:        %s\n", project.Version())
	_, _ = fmt.Fprintf(r.stdout, "Git Commit:     %s\n", project.GitSHA())
	_, _ = fmt.Fprintf(r.stdout, "Go Version:     %s\n", runtime.Version())
	_, _ = fmt.Fprintf(r.stdout, "OS / Arch:      %s / %s\n", runtime.GOOS, runtime.GOARCH)
	_, _ = fmt.Fprintf(r.stdout, "Source:         %s\n", project.Source())

	return nil
}
