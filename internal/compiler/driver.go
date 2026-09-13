package compiler

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// BuildOptions controls output and linker arguments passed to TinyCC.
type BuildOptions struct {
	Output, TCC                                          string
	KeepC, Verbose                                       bool
	Includes, LibraryDirs, Libraries, Defines, ExtraArgs []string
	Stderr                                               io.Writer
}

// Build materializes generated C and invokes TinyCC.
func Build(c string, o BuildOptions) error {
	if o.TCC == "" {
		o.TCC = "tcc"
	}
	f, err := os.CreateTemp(filepath.Dir(o.Output), ".rubyc-*.c")
	if err != nil {
		return err
	}
	cfile := f.Name()
	defer func() {
		if !o.KeepC {
			_ = os.Remove(cfile)
		}
	}()
	if _, err = f.WriteString(c); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if o.KeepC {
		if err = os.Rename(cfile, o.Output+".c"); err != nil {
			return err
		}
		cfile = o.Output + ".c"
	}
	args := []string{"-o", o.Output}
	for _, x := range o.Includes {
		args = append(args, "-I"+x)
	}
	for _, x := range o.LibraryDirs {
		args = append(args, "-L"+x)
	}
	for _, x := range o.Defines {
		args = append(args, "-D"+x)
	}
	for _, x := range o.Libraries {
		args = append(args, "-l"+x)
	}
	args = append(args, o.ExtraArgs...)
	args = append(args, cfile)
	if o.Verbose {
		fmt.Fprintln(o.Stderr, o.TCC, strings.Join(args, " "))
	}
	cmd := exec.Command(o.TCC, args...)
	cmd.Stdout = o.Stderr
	cmd.Stderr = o.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("TinyCC failed: %w", err)
	}
	return nil
}
