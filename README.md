# rubyc

`rubyc` is an experimental Ruby-to-C compiler driver written in Go.  It parses a
small, deliberately specified Ruby subset, lowers it to portable C, and invokes
TinyCC (`tcc`) to produce a native executable.

## Quick start

```sh
go build ./cmd/rubyc
cat > hello.rb <<'RUBY'
name = "Ruby"
puts "Hello, " + name
RUBY
./rubyc hello.rb -o hello.exe
./hello.exe
```

`tcc` must be installed or supplied with `--tcc /path/to/tcc`.  `--emit-c` is
useful when TinyCC is unavailable.

## Supported Ruby subset

This first compiler slice supports `puts`, `print`, reassignment, string and
integer literals, `true`/`false`/`nil`, parentheses, `+`, `-`, `*`, `/`,
numeric comparisons, `==`/`!=`, `!`, `if`/`unless`/`else`/`end`, `while`, and
line comments. Selected standard-library-compatible calls are also available:
`gets`, `String#length`/`size`, `upcase`, `downcase`, `chomp`, and `to_s`.
It intentionally rejects constructs it cannot compile instead of
silently changing Ruby semantics.

Ruby 4.0 compatibility and the complete standard library are future work; this
repository does **not** claim to implement them yet.

## CLI

```
rubyc [options] program.rb
  -o FILE          executable output path (default: input basename)
  --emit-c FILE    write generated C and do not invoke a compiler
  --keep-c         retain generated C beside the output
  --tcc PATH       TinyCC executable (default: tcc or RUBYC_TCC)
  -I DIR           add C include directory (repeatable)
  -L DIR           add library directory (repeatable)
  -l NAME          link library (repeatable)
  -D NAME[=VALUE]  define C preprocessor macro (repeatable)
  --cc-arg ARG     pass one additional option to TinyCC (repeatable)
  --verbose        show compiler invocation
  --version        show version
```
