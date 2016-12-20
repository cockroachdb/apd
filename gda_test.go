// Copyright 2016 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package apd

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const testDir = "testdata"

var (
	flagPython     = flag.Bool("python", false, "check if apd's results are identical to python; print an ignore line if they are")
	flagSummary    = flag.Bool("summary", false, "print a summary")
	flagFailFast   = flag.Bool("fast", false, "stop work after first error; disables parallel testing")
	flagIgnore     = flag.Bool("ignore", false, "print ignore lines on errors; disables parallel testing")
	flagNoParallel = flag.Bool("noparallel", false, "disables parallel testing")
)

type TestCase struct {
	Precision                int
	MaxExponent, MinExponent int
	Rounding                 string
	Extended, Clamp          bool

	ID         string
	Operation  string
	Operands   []string
	Result     string
	Conditions []string
}

func (tc TestCase) HasNull() bool {
	if tc.Result == "#" {
		return true
	}
	for _, o := range tc.Operands {
		if o == "#" {
			return true
		}
	}
	return false
}

func ParseDecTest(r io.Reader) ([]TestCase, error) {
	scanner := bufio.NewScanner(r)

	tc := TestCase{
		Extended: true,
	}
	var err error

	var res []TestCase

	for scanner.Scan() {
		text := scanner.Text()
		line := strings.Fields(strings.ToLower(text))
		for i, t := range line {
			if strings.HasPrefix(t, "--") {
				line = line[:i]
				break
			}
		}
		if len(line) == 0 {
			continue
		}
		if strings.HasSuffix(line[0], ":") {
			if len(line) != 2 {
				return nil, fmt.Errorf("expected 2 tokens, got %q", text)
			}
			switch directive := line[0]; directive[:len(directive)-1] {
			case "precision":
				tc.Precision, err = strconv.Atoi(line[1])
				if err != nil {
					return nil, err
				}
			case "maxexponent":
				tc.MaxExponent, err = strconv.Atoi(line[1])
				if err != nil {
					return nil, err
				}
			case "minexponent":
				tc.MinExponent, err = strconv.Atoi(line[1])
				if err != nil {
					return nil, err
				}
			case "rounding":
				tc.Rounding = line[1]
			case "version":
				// ignore
			case "extended":
				tc.Extended = line[1] == "1"
			case "clamp":
				tc.Clamp = line[1] == "1"
			default:
				return nil, fmt.Errorf("unsupported directive: %s", directive)
			}
		} else {
			if len(line) < 5 {
				return nil, fmt.Errorf("short test case line: %q", text)
			}
			tc.ID = line[0]
			tc.Operation = line[1]
			tc.Operands = nil
			var ops []string
			line = line[2:]
			for i, o := range line {
				if o == "->" {
					tc.Operands = ops
					line = line[i+1:]
					break
				}
				o = cleanNumber(o)
				ops = append(ops, o)
			}
			if tc.Operands == nil || len(line) < 1 {
				return nil, fmt.Errorf("bad test case line: %q", text)
			}
			tc.Result = cleanNumber(line[0])
			tc.Conditions = line[1:]
			res = append(res, tc)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

func cleanNumber(s string) string {
	if len(s) > 1 && s[0] == '\'' && s[len(s)-1] == '\'' {
		s = s[1 : len(s)-1]
		s = strings.Replace(s, `''`, `'`, -1)
	} else if len(s) > 1 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
		s = strings.Replace(s, `""`, `"`, -1)
	}
	return s
}

func TestParseDecTest(t *testing.T) {
	files, err := ioutil.ReadDir(testDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, fi := range files {
		t.Run(fi.Name(), func(t *testing.T) {
			f, err := os.Open(filepath.Join(testDir, fi.Name()))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			_, err = ParseDecTest(f)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGDA(t *testing.T) {
	files := []string{
		"abs0",
		"add0",
		"compare0",
		"divide0",
		"divideint0",
		"exp0",
		"ln0",
		"log100",
		"minus0",
		"multiply0",
		"plus0",
		"power0",
		"remainder0",
		"rounding0",
		"squareroot0",
		"subtract0",
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%10s%8s%8s%8s%8s%8s%8s\n", "name", "total", "success", "fail", "ignore", "skip", "missing")
	for _, fname := range files {
		succeed := t.Run(fname, func(t *testing.T) {
			ignored, skipped, success, fail, total := gdaTest(t, fname)
			missing := total - ignored - skipped - success - fail
			if *flagSummary {
				fmt.Fprintf(&buf, "%10s%8d%8d%8d%8d%8d%8d\n",
					fname,
					total,
					success,
					fail,
					ignored,
					skipped,
					missing,
				)
				if missing != 0 {
					t.Fatalf("unaccounted summary result: missing: %d, total: %d, %d, %d, %d", missing, total, ignored, skipped, success)
				}
			}
		})
		if !succeed && *flagFailFast {
			break
		}
	}
	if *flagSummary {
		fmt.Print(buf.String())
	}
}

func gdaTest(t *testing.T, name string) (int, int, int, int, int) {
	path := filepath.Join(testDir, name+".decTest")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tcs, err := ParseDecTest(f)
	if err != nil {
		t.Fatal(err)
	}
	var lock sync.Mutex
	var ignored, skipped, success, fail, total int
	for _, tc := range tcs {
		tc := tc
		succeed := t.Run(tc.ID, func(t *testing.T) {
			defer func() {
				lock.Lock()
				total++
				if GDAignore[tc.ID] {
					ignored++
				} else if t.Skipped() {
					skipped++
				} else if t.Failed() {
					fail++
					if *flagIgnore {
						tc.PrintIgnore()
					}
				} else {
					success++
				}
				lock.Unlock()
			}()
			if GDAignore[tc.ID] {
				t.Skip("ignored")
			}
			if tc.HasNull() {
				t.Skip("has null")
			}
			if !*flagNoParallel && !*flagFailFast && !*flagIgnore {
				t.Parallel()
			}
			t.Logf("%s:/%s", path, tc.ID)
			mode, ok := rounders[tc.Rounding]
			if !ok {
				t.Fatalf("unsupported rounding mode %s", tc.Rounding)
			}
			operands := make([]*Decimal, len(tc.Operands))
			for i, o := range tc.Operands {
				operands[i] = newDecimal(t, o)
			}
			d := new(Decimal)
			c := Context{
				Precision:   uint32(tc.Precision),
				MaxExponent: int32(tc.MaxExponent),
				MinExponent: int32(tc.MinExponent),
				Rounding:    mode,
			}
			// helpful acme address link
			t.Logf("%s %s = %s (prec: %d, round: %s)", tc.Operation, strings.Join(tc.Operands, " "), tc.Result, tc.Precision, tc.Rounding)
			start := time.Now()
			defer func() {
				t.Logf("duration: %s", time.Since(start))
			}()

			done := make(chan error, 1)
			var err error
			go func() {
				switch tc.Operation {
				case "abs":
					err = c.Abs(d, operands[0])
				case "add":
					err = c.Add(d, operands[0], operands[1])
				case "compare":
					var c int
					c, err = operands[0].Cmp(operands[1])
					d.SetInt64(int64(c))
				case "divide":
					err = c.Quo(d, operands[0], operands[1])
				case "divideint":
					err = c.QuoInteger(d, operands[0], operands[1])
				case "exp":
					err = c.Exp(d, operands[0])
				case "ln":
					err = c.Ln(d, operands[0])
				case "log10":
					err = c.Log10(d, operands[0])
				case "minus":
					err = c.Neg(d, operands[0])
				case "multiply":
					err = c.Mul(d, operands[0], operands[1])
				case "plus":
					err = c.Add(d, operands[0], decimalZero)
				case "power":
					err = c.Pow(d, operands[0], operands[1])
				case "remainder":
					err = c.Rem(d, operands[0], operands[1])
				case "squareroot":
					err = c.Sqrt(d, operands[0])
				case "subtract":
					err = c.Sub(d, operands[0], operands[1])
				default:
					done <- fmt.Errorf("unknown operation: %s", tc.Operation)
				}
				done <- nil
			}()
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second * 120):
				t.Fatalf("timeout")
			}
			// Verify the operands didn't change.
			for i, o := range tc.Operands {
				v := newDecimal(t, o)
				if c, err := v.Cmp(operands[i]); err != nil {
					t.Fatal(err)
				} else if c != 0 {
					t.Fatalf("operand %d changed from %s to %s", i, o, operands[i])
				}
			}
			if tc.Result == "?" {
				if err != nil {
					return
				}
				t.Fatalf("expected error, got %#v", d)
			}
			if err != nil {
				testExponentError(t, err)
				if *flagPython {
					if tc.CheckPython(t, d) {
						return
					}
				}
				t.Fatalf("%+v", err)
			}
			r := newDecimal(t, tc.Result)
			p, err := d.Cmp(r)
			if err != nil {
				t.Fatal(err)
			}
			if p != 0 {
				if *flagPython {
					if tc.CheckPython(t, d) {
						return
					}
				}
				t.Logf("result: %s", d)
				t.Fatalf("got: %#v", d)
			}
		})
		if !succeed {
			if *flagFailFast {
				break
			}
		} else {
			success++
		}
	}
	success -= ignored + skipped
	return ignored, skipped, success, fail, total
}

// CheckPython returns true if python outputs d for this test case. It prints
// an ignore line if true.
func (tc TestCase) CheckPython(t *testing.T, d *Decimal) (ok bool) {
	const tmpl = `from decimal import *
getcontext().prec = %d
getcontext().rounding = 'ROUND_%s'
print %s`

	var op string
	switch tc.Operation {
	case "add":
		op = "+"
	case "divide":
		op = "/"
	case "divideint":
		op = "//"
	case "exp":
		op = "exp"
	case "ln":
		op = "ln"
	case "log10":
		op = "log10"
	case "multiply":
		op = "*"
	case "power":
		op = "**"
	case "remainder":
		op = "%"
	case "squareroot":
		op = "sqrt"
	case "subtract":
		op = "-"
	default:
		t.Fatalf("unknown operator: %s", tc.Operation)
	}
	var line string
	switch len(tc.Operands) {
	case 1:
		line = fmt.Sprintf("Decimal('%s').%s()", tc.Operands[0], op)
	case 2:
		line = fmt.Sprintf("Decimal('%s') %s Decimal('%s')", tc.Operands[0], op, tc.Operands[1])
	default:
		t.Fatalf("unknown operands: %d", len(tc.Operands))
	}

	script := fmt.Sprintf(tmpl, tc.Precision, strings.ToUpper(tc.Rounding), line)
	out, err := exec.Command("python", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %s", err, out)
	}
	so := strings.TrimSpace(string(out))
	r := newDecimal(t, so)
	c, err := d.Cmp(r)
	if err != nil {
		t.Fatal(err)
	}
	if c != 0 {
		t.Errorf("python's result: %s", so)
	} else {
		// python and apd agree, print ignore line
		tc.PrintIgnore()
	}

	return c == 0
}

func (tc TestCase) PrintIgnore() {
	fmt.Printf("	\"%s\": true,\n", tc.ID)
}

var GDAignore = map[string]bool{

	// --- These do not need not be fixed.

	// python-identical results
	// Both apd and python disagree with GDA in these cases.
	"add303":  true,
	"add307":  true,
	"add642":  true,
	"add643":  true,
	"add644":  true,
	"add651":  true,
	"add652":  true,
	"add653":  true,
	"add662":  true,
	"add663":  true,
	"add664":  true,
	"add671":  true,
	"add672":  true,
	"add673":  true,
	"add682":  true,
	"add683":  true,
	"add684":  true,
	"add691":  true,
	"add692":  true,
	"add693":  true,
	"add702":  true,
	"add703":  true,
	"add704":  true,
	"add711":  true,
	"add712":  true,
	"add713":  true,
	"div412":  true,
	"dvi411":  true,
	"ln0054":  true,
	"ln116":   true,
	"log1308": true,
	"log1322": true,
	"log1323": true,
	"log1325": true,
	"log1326": true,
	"rem071":  true,
	"rem343":  true,
	"rem344":  true,
	"rem348":  true,
	"rem349":  true,
	"rem372":  true,
	"rem507":  true,
	"rem509":  true,
	"rem511":  true,
	"sqtx763": true,
	"sqtx764": true,
	"sub053":  true,
	"sub062":  true,
	"sub063":  true,
	"sub067":  true,
	"sub068":  true,
	"sub080":  true,
	"sub120":  true,
	"sub121":  true,
	"sub122":  true,
	"sub123":  true,
	"sub124":  true,
	"sub126":  true,
	"sub127":  true,
	"sub128":  true,
	"sub129":  true,
	"sub130":  true,
	"sub131":  true,
	"sub132":  true,
	"sub133":  true,
	"sub134":  true,
	"sub135":  true,
	"sub137":  true,
	"sub138":  true,
	"sub139":  true,
	"sub140":  true,
	"sub141":  true,
	"sub142":  true,
	"sub143":  true,
	"sub150":  true,
	"sub151":  true,
	"sub152":  true,
	"sub153":  true,
	"sub311":  true,
	"sub332":  true,
	"sub333":  true,
	"sub342":  true,
	"sub343":  true,
	"sub363":  true,
	"sub670":  true,
	"sub671":  true,
	"sub672":  true,
	"sub673":  true,
	"sub674":  true,
	"sub680":  true,
	"sub681":  true,
	"sub682":  true,
	"sub683":  true,
	"sub684":  true,
	"sub685":  true,
	"sub686":  true,
	"sub687":  true,
	"sub688":  true,
	"sub689":  true,
	"sub690":  true,
	"sub691":  true,
	"sub692":  true,
	"sub693":  true,
	"sub694":  true,
	"sub703":  true,
	"sub707":  true,
	"sub910":  true,
	"sub911":  true,
	"sub922":  true,
	"sub923":  true,
	"sub926":  true,
	"sub927":  true,
	"sub928":  true,
	"sub929":  true,
	"sub930":  true,
	"sub932":  true,
	"sub934":  true,
	"sub935":  true,
	"sub936":  true,
	"sub937":  true,
	"sub938":  true,
	"sub939":  true,
	"sub940":  true,
	"sub941":  true,
	"sub942":  true,
	"sub943":  true,
	"sub944":  true,
	"sub945":  true,
	"sub946":  true,
	"sub947":  true,

	// invalid operation errors; most test harnesses probably skip this
	"log901": true,

	// invalid context errors: unsure what these are testing
	"ln903":  true,
	"ln905":  true,
	"log903": true,
	"log905": true,

	// --- These should be fixed.
	// TODO(mjibson): fix these broken tests

	// lost digits
	"dvi072": true,
	"dvi073": true,
	"dvi074": true,
	"rem072": true,
	"rem073": true,
	"rem074": true,

	// undetected overflow
	"sqtx765": true,
	"sqtx766": true,

	// undetected underflow
	"exp048": true,
	"exp755": true,
	"exp756": true,
	"exp757": true,
	"exp759": true,
	"exp760": true,
	"exp761": true,
	"exp762": true,
	"exp763": true,
	"exp764": true,
	"exp765": true,
	"exp766": true,
	"exp769": true,
	"exp770": true,
	"exp771": true,
	"exp772": true,
	"exp773": true,
	"exp774": true,

	// should fail with too much precision needed
	"pow253": true,
	"pow254": true,

	// 1** big integer, should return 1 instead of fail
	"pow2011": true,
	"pow2012": true,
	"pow2013": true,
	"pow2014": true,
	"pow2041": true,
	"pow2042": true,
	"pow2043": true,
	"pow2044": true,
	"pow2045": true,
	"pow2046": true,
	"pow2047": true,
	"pow2048": true,
	"pow2049": true,
	"pow2051": true,
	"pow2052": true,
	"pow2053": true,
	"pow2054": true,
	"pow2055": true,
	"pow2056": true,

	// incorrect rounding
	"rpo107": true,
	"rpo213": true,
	"rpo412": true,
	"rpo507": true,
	"rpo607": true,
	"rpo707": true,
}
