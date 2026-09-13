package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rubyc/rubyc/internal/compiler"
)

const version = "0.1.0"

type stringsFlag []string

func (s *stringsFlag) String() string     { return fmt.Sprint([]string(*s)) }
func (s *stringsFlag) Set(v string) error { *s = append(*s, v); return nil }

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "rubyc:", err)
		os.Exit(1)
	}
}

func run(args []string, out, errOut io.Writer) error {
	// Go's flag package stops parsing at the first positional argument. Ruby
	// users conventionally write `rubyc app.rb -o app`, so move the source to
	// the end before parsing while leaving every option and its value intact.
	for i, arg := range args {
		if strings.HasSuffix(arg, ".rb") && !strings.HasPrefix(arg, "-") {
			args = append(append(args[:i:i], args[i+1:]...), arg)
			break
		}
	}
	fs := flag.NewFlagSet("rubyc", flag.ContinueOnError)
	fs.SetOutput(errOut)
	output := fs.String("o", "", "output executable")
	emitC := fs.String("emit-c", "", "write C output only")
	keepC := fs.Bool("keep-c", false, "retain generated C")
	tcc := fs.String("tcc", os.Getenv("RUBYC_TCC"), "TinyCC executable")
	verbose := fs.Bool("verbose", false, "print compiler invocation")
	showVersion := fs.Bool("version", false, "print version")
	var includes, libs, linkLibs, defines, ccArgs stringsFlag
	fs.Var(&includes, "I", "C include directory")
	fs.Var(&libs, "L", "C library directory")
	fs.Var(&linkLibs, "l", "C library name")
	fs.Var(&defines, "D", "C preprocessor definition")
	fs.Var(&ccArgs, "cc-arg", "additional TinyCC argument")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Fprintln(out, "rubyc", version)
		return nil
	}
	if fs.NArg() != 1 {
		return errors.New("expected exactly one Ruby source file")
	}
	input := fs.Arg(0)
	source, err := os.ReadFile(input)
	if err != nil {
		return err
	}
	c, err := compiler.Compile(string(source), input)
	if err != nil {
		return err
	}
	if *emitC != "" {
		return os.WriteFile(*emitC, []byte(c), 0o644)
	}
	if *output == "" {
		base := filepath.Base(input)
		*output = base[:len(base)-len(filepath.Ext(base))]
	}
	return compiler.Build(c, compiler.BuildOptions{Output: *output, TCC: *tcc, KeepC: *keepC, Verbose: *verbose, Includes: includes, LibraryDirs: libs, Libraries: linkLibs, Defines: defines, ExtraArgs: ccArgs, Stderr: errOut})
}
