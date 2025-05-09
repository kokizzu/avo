package gotypes

import (
	"go/token"
	"go/types"
	"regexp"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestLookupSignature(t *testing.T) {
	pkg := LoadPackageTypes(t, "math")
	s, err := LookupSignature(pkg, "Frexp")
	if err != nil {
		t.Fatal(err)
	}

	expect, err := ParseSignature("func(f float64) (frac float64, exp int)")
	if err != nil {
		t.Fatal(err)
	}

	if s.String() != expect.String() {
		t.Errorf("\n   got: %s\nexpect: %s\n", s, expect)
	}
}

func TestLookupSignatureErrors(t *testing.T) {
	cases := []struct {
		PackagePath   string
		FunctionName  string
		ExpectedError string
	}{
		{"runtime", "HmmIdk", "could not find function \"HmmIdk\""},
		{"crypto", "Decrypter", "object \"Decrypter\" does not have signature type"},
		{"encoding/base64", "StdEncoding", "object \"StdEncoding\" does not have signature type"},
	}
	for _, c := range cases {
		pkg := LoadPackageTypes(t, c.PackagePath)
		_, err := LookupSignature(pkg, c.FunctionName)
		if err == nil {
			t.Fatalf("expected error looking up '%s' in package '%s'", c.FunctionName, c.PackagePath)
		}
		if err.Error() != c.ExpectedError {
			t.Fatalf("wrong error message\n   got: %q\nexpect: %q", err.Error(), c.ExpectedError)
		}
	}
}

func LoadPackageTypes(t *testing.T, path string) *types.Package {
	t.Helper()
	cfg := &packages.Config{
		Mode: packages.NeedTypes | packages.NeedDeps | packages.NeedImports,
	}
	pkgs, err := packages.Load(cfg, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 {
		t.Fatal("expected to load exactly one package")
	}
	return pkgs[0].Types
}

func TestParseSignature(t *testing.T) {
	cases := []struct {
		Expr         string
		ExpectParams *types.Tuple
		ExpectReturn *types.Tuple
	}{
		{
			Expr: "func()",
		},
		{
			Expr: "func(x, y uint64)",
			ExpectParams: types.NewTuple(
				types.NewParam(token.NoPos, nil, "x", types.Typ[types.Uint64]),
				types.NewParam(token.NoPos, nil, "y", types.Typ[types.Uint64]),
			),
		},
		{
			Expr: "func(n int, s []string) byte",
			ExpectParams: types.NewTuple(
				types.NewParam(token.NoPos, nil, "n", types.Typ[types.Int]),
				types.NewParam(token.NoPos, nil, "s", types.NewSlice(types.Typ[types.String])),
			),
			ExpectReturn: types.NewTuple(
				types.NewParam(token.NoPos, nil, "", types.Typ[types.Byte]),
			),
		},
		{
			Expr: "func(x, y int) (x0, y0 int, s string)",
			ExpectParams: types.NewTuple(
				types.NewParam(token.NoPos, nil, "x", types.Typ[types.Int]),
				types.NewParam(token.NoPos, nil, "y", types.Typ[types.Int]),
			),
			ExpectReturn: types.NewTuple(
				types.NewParam(token.NoPos, nil, "x0", types.Typ[types.Int]),
				types.NewParam(token.NoPos, nil, "y0", types.Typ[types.Int]),
				types.NewParam(token.NoPos, nil, "s", types.Typ[types.String]),
			),
		},
	}
	for _, c := range cases {
		s, err := ParseSignature(c.Expr)
		if err != nil {
			t.Fatal(err)
		}
		if !TypesTuplesEqual(s.sig.Params(), c.ExpectParams) {
			t.Errorf("parameter mismatch\ngot %#v\nexpect %#v\n", s.sig.Params(), c.ExpectParams)
		}
		if !TypesTuplesEqual(s.sig.Results(), c.ExpectReturn) {
			t.Errorf("return value(s) mismatch\ngot %#v\nexpect %#v\n", s.sig.Results(), c.ExpectReturn)
		}
	}
}

func TestParseSignatureErrors(t *testing.T) {
	cases := []struct {
		Expr         string
		ErrorPattern string
	}{
		{"idkjklol", `(undeclared|undefined)`}, // error message changed in go 1.20
		{"struct{}", `not a function signature`},
		{"uint32(0xfeedbeef)", `should have nil value`},
	}
	for _, c := range cases {
		errrx, err := regexp.Compile(c.ErrorPattern)
		if err != nil {
			t.Fatal(err)
		}

		s, err := ParseSignature(c.Expr)
		if s != nil || err == nil || !errrx.MatchString(err.Error()) {
			t.Errorf("expect error from expression %s\ngot: %s\nexpect match: %s\n", c.Expr, err, c.ErrorPattern)
		}
	}
}

func TypesTuplesEqual(a, b *types.Tuple) bool {
	if a.Len() != b.Len() {
		return false
	}
	n := a.Len()
	for i := range n {
		if !TypesVarsEqual(a.At(i), b.At(i)) {
			return false
		}
	}
	return true
}

func TypesVarsEqual(a, b *types.Var) bool {
	return a.Name() == b.Name() && types.Identical(a.Type(), b.Type())
}

func TestSignatureSizes(t *testing.T) {
	cases := []struct {
		Expr    string
		ArgSize int
	}{
		{"func()", 0},
		{"func(uint64) uint64", 16},
		{"func([7]byte) byte", 9},
		{"func(uint64, uint64) (uint64, uint64)", 32},
		{"func(uint16)", 2},           // issue #191
		{"func(*uint64, uint32)", 12}, // issue #195
	}
	for _, c := range cases {
		s, err := ParseSignature(c.Expr)
		if err != nil {
			t.Fatal(err)
		}
		if s.Bytes() != c.ArgSize {
			t.Errorf("%s: size %d expected %d", s, s.Bytes(), c.ArgSize)
		}
	}
}
