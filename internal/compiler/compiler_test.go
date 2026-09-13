package compiler

import (
	"strings"
	"testing"
)

func TestCompileHelloAndConditional(t *testing.T) {
	c, err := Compile("name = \"Ruby\"\nname = name + \" 4\"\nputs \"Hello, \" + name\nif 2 + 3 * 4 == 14\n  puts 2 + 3\nelse\n  print \"no\"\nend\n", "hello.rb")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Value name=str(\"Ruby\")", "name=add(name,str(\" 4\"))", "out(add(str(\"Hello, \"),name),1)", "if (truth(num(eq("} {
		if !strings.Contains(c, want) {
			t.Errorf("generated C missing %q", want)
		}
	}
}
func TestCompileWhileUnlessAndLiterals(t *testing.T) {
	c, err := Compile("i = 0\nwhile i < 3\n puts i\n i = i + 1\nend\nunless false\n puts nil\nend\n", "loop.rb")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"while (truth(num((i).n<(num(3)).n)))", "if (truth(num(!truth(num(0)))))", "out(nil(),1)"} {
		if !strings.Contains(c, want) {
			t.Errorf("generated C missing %q", want)
		}
	}
}
func TestCompileSelectedStandardLibraryMethods(t *testing.T) {
	c, err := Compile("word = \"Ruby\"\nputs word.upcase\nputs word.downcase\nputs word.length\nputs 42.to_s\nname = gets.chomp\nputs name\n", "stdlib.rb")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"upper(word)", "lower(word)", "length(word)", "str(text(num(42)))", "chomp(input())"} {
		if !strings.Contains(c, want) {
			t.Errorf("generated C missing %q", want)
		}
	}
}
func TestCompileRejectsUnknownValue(t *testing.T) {
	_, err := Compile("puts missing\n", "bad.rb")
	if err == nil || !strings.Contains(err.Error(), "unknown value") {
		t.Fatalf("unexpected error: %v", err)
	}
}
