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
		line := strings.Fields(text)
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
			switch directive := line[0]; strings.ToLower(directive[:len(directive)-1]) {
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
		"base0",
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
			switch tc.Operation {
			case "toEng":
				t.Skip("unsupported")
			}
			if !*flagNoParallel && !*flagFailFast && !*flagIgnore {
				t.Parallel()
			}
			// helpful acme address link
			t.Logf("%s:/^%s", path, tc.ID)
			t.Logf("%s %s = %s (prec: %d, round: %s, Emax: %d, Emin: %d)", tc.Operation, strings.Join(tc.Operands, " "), tc.Result, tc.Precision, tc.Rounding, tc.MaxExponent, tc.MinExponent)
			mode, ok := rounders[tc.Rounding]
			if !ok || mode == nil {
				t.Fatalf("unsupported rounding mode %s", tc.Rounding)
			}
			operands := make([]*Decimal, len(tc.Operands))
			c := &Context{
				Precision:   uint32(tc.Precision),
				MaxExponent: int32(tc.MaxExponent),
				MinExponent: int32(tc.MinExponent),
				Rounding:    mode,
			}
			for i, o := range tc.Operands {
				d, err := c.NewFromString(o)
				if err != nil {
					testExponentError(t, err)
					if tc.Result == "?" {
						return
					}
					t.Fatalf("%+v", err)
				}
				operands[i] = d
			}
			var s string
			d := new(Decimal)
			start := time.Now()
			defer func() {
				t.Logf("duration: %s", time.Since(start))
			}()

			done := make(chan error, 1)
			var err error
			go func() {
				switch strings.ToLower(tc.Operation) {
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
				case "tosci":
					s = operands[0].ToSci()
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
				v := newDecimal(t, c, o)
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
				if *flagPython {
					if tc.CheckPython(t, d) {
						return
					}
				}
				t.Fatalf("expected error, got %s", d)
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
			switch strings.ToLower(tc.Operation) {
			case "tosci", "toeng":
				if s != tc.Result {
					t.Fatalf("expected %s, got %s", tc.Result, s)
				}
				return
			}
			r := newDecimal(t, testCtx, tc.Result)
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

var rounders = map[string]Rounder{
	"ceiling":   RoundCeiling,
	"down":      RoundDown,
	"floor":     RoundFloor,
	"half_down": RoundHalfDown,
	"half_even": RoundHalfEven,
	"half_up":   RoundHalfUp,
	"up":        RoundUp,
}

// CheckPython returns true if python outputs d for this test case. It prints
// an ignore line if true.
func (tc TestCase) CheckPython(t *testing.T, d *Decimal) (ok bool) {
	const tmpl = `from decimal import *
c = getcontext()
c.prec=%d
c.rounding='ROUND_%s'
c.Emax=%d
c.Emin=%d
print %s`

	var op string
	switch strings.ToLower(tc.Operation) {
	case "abs":
		op = "abs"
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
	case "tosci":
		op = "to_sci_string"
	default:
		t.Fatalf("unknown operator: %s", tc.Operation)
	}
	var line string
	switch len(tc.Operands) {
	case 1:
		line = fmt.Sprintf("c.%s(c.create_decimal('%s'))", op, tc.Operands[0])
	case 2:
		line = fmt.Sprintf("c.create_decimal('%s') %s c.create_decimal('%s')", tc.Operands[0], op, tc.Operands[1])
	default:
		t.Fatalf("unknown operands: %d", len(tc.Operands))
	}

	script := fmt.Sprintf(tmpl, tc.Precision, strings.ToUpper(tc.Rounding), tc.MaxExponent, tc.MinExponent, line)
	out, err := exec.Command("python", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %s", err, out)
	}
	so := strings.TrimSpace(string(out))
	r := newDecimal(t, testCtx, so)
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
	// python-identical results. (Both apd and python disagree with GDA in these
	// cases.)
	"add642": true,
	"add643": true,
	"add644": true,
	"add651": true,
	"add652": true,
	"add653": true,
	"add662": true,
	"add663": true,
	"add664": true,
	"add671": true,
	"add672": true,
	"add673": true,
	"add682": true,
	"add683": true,
	"add684": true,
	"add691": true,
	"add692": true,
	"add693": true,
	"add702": true,
	"add703": true,
	"add704": true,
	"add711": true,
	"add712": true,
	"add713": true,
	"ln116":  true,
	"ln903":  true,
	"ln905":  true,
	"log903": true,
	"log905": true,
	"sub062": true,
	"sub063": true,
	"sub067": true,
	"sub068": true,
	"sub080": true,
	"sub142": true,
	"sub143": true,
	"sub332": true,
	"sub333": true,
	"sub342": true,
	"sub343": true,
	"sub363": true,
	"sub910": true,
	"sub911": true,
	"sub922": true,
	"sub923": true,
	"sub926": true,
	"sub927": true,
	"sub928": true,
	"sub929": true,
	"sub930": true,
	"sub932": true,
	"sub934": true,
	"sub936": true,
	"sub937": true,
	"sub938": true,
	"sub939": true,
	"sub940": true,
	"sub941": true,
	"sub942": true,
	"sub943": true,
	"sub944": true,
	"sub945": true,
	"sub946": true,
	"sub947": true,

	// GDA thinks these shouldn't over or underflow, but python does
	"ln0901":  true,
	"ln0902":  true,
	"log0001": true,
	"log0020": true,
	"log1146": true,
	"log1147": true,
	"log1156": true,
	"log1157": true,
	"log1166": true,
	"log1167": true,

	// GDA thinks these aren't subnormal, but python does
	"ln759": true,
	"ln760": true,
	"ln761": true,
	"ln762": true,
	"ln763": true,
	"ln764": true,
	"ln765": true,
	"ln766": true,

	// TODO(mjibson): fix tests below

	// incorrect rounding
	"rpo213": true,
	"rpo412": true,
}
