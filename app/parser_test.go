package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestParameterExpansion(t *testing.T) {
	original := shellVariables
	shellVariables = map[string]string{
		"Variable_1": "Value_1",
		"Variable_2": "Value2",
		"spaces":     "two words",
		"literal":    `$Variable_1 "quoted"`,
		"empty":      "",
	}
	t.Cleanup(func() { shellVariables = original })
	for _, test := range []struct {
		line string
		want []string
	}{
		{`echo $Variable_1 $Variable_2`, []string{"echo", "Value_1", "Value2"}},
		{`echo ${Variable_1}end`, []string{"echo", "Value_1end"}},
		{`echo ${Variable_1}and${Variable_2}`, []string{"echo", "Value_1andValue2"}},
		{`echo $Variable_1end ${Variable_1}end`, []string{"echo", "", "Value_1end"}},
		{`echo stock_${Variable_1}_id "${spaces}"`, []string{"echo", "stock_Value_1_id", "two words"}},
		{`echo '${Variable_1}' \${Variable_1} "\${Variable_1}"`, []string{"echo", "${Variable_1}", "${Variable_1}", "${Variable_1}"}},
		{`echo ${empty} "${missing}" ${missing}end`, []string{"echo", "", "", "end"}},
		{`echo $spaces`, []string{"echo", "two words"}},
		{`echo "$spaces"`, []string{"echo", "two words"}},
		{`echo '$Variable_1' \$Variable_1 "\$Variable_1"`, []string{"echo", "$Variable_1", "$Variable_1", "$Variable_1"}},
		{`echo prefix-$Variable_1/$Variable_2`, []string{"echo", "prefix-Value_1/Value2"}},
		{`echo $literal`, []string{"echo", `$Variable_1 "quoted"`}},
		{`echo $empty "$missing" end`, []string{"echo", "", "", "end"}},
		{`echo $ $23`, []string{"echo", "$", "$23"}},
		{`echo 'prefix-'"$Variable_1"-suffix`, []string{"echo", "prefix-Value_1-suffix"}},
		{`echo "" '' end`, []string{"echo", "", "", "end"}},
		{`echo "a\\b\"c" one\ two`, []string{"echo", "a\\b\"c", "one two"}},
		{`echo "hello 'world'" 'hello "world"'`, []string{"echo", "hello 'world'", `hello "world"`}},
		{`echo café`, []string{"echo", "café"}},
	} {
		got, err := getModifiedInput(test.line)
		if err != nil || !slices.Equal(got, test.want) {
			t.Errorf("parse %q = %q, %v; want %q", test.line, got, err, test.want)
		}
	}
}

func TestParameterExpansionDispatch(t *testing.T) {
	original := shellVariables
	shellVariables = make(map[string]string)
	t.Cleanup(func() { shellVariables = original })
	for _, line := range []string{`declare Variable_1=Value_1`, `declare Variable_2='two words'`} {
		result, stdout, stderr := runTestLine(line)
		if result.Status != 0 || stdout != "" || stderr != "" {
			t.Fatalf("%s = %+v, stdout %q, stderr %q", line, result, stdout, stderr)
		}
	}
	script := filepath.Join(t.TempDir(), "show-args")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s\\n' \"$#\" \"$1\" \"$2\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ line, want string }{
		{`echo $Variable_1 $Variable_2`, "Value_1 two words\n"},
		{script + ` $Variable_1 $Variable_2`, "2\nValue_1\ntwo words\n"},
		{`echo ${Variable_1}end ${Variable_1}and${Variable_2}`, "Value_1end Value_1andtwo words\n"},
		{script + ` stock_${Variable_1}_id ${Variable_2}`, "2\nstock_Value_1_id\ntwo words\n"},
		{`declare -p Variable_2`, "declare -- Variable_2=\"two words\"\n"},
	} {
		result, stdout, stderr := runTestLine(test.line)
		if result.Status != 0 || stdout != test.want || stderr != "" {
			t.Errorf("%s = %+v, stdout %q, stderr %q; want %q", test.line, result, stdout, stderr, test.want)
		}
	}
}
