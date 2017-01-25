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
	flagIgnore     = flag.Bool("ignore", false, "print ignore lines on errors")
	flagNoParallel = flag.Bool("noparallel", false, "disables parallel testing")
	flagTime       = flag.Duration("time", 0, "interval at which to print long-running functions; 0 disables")
)

// REVIEW: for now I'm not going to review this.

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

func (tc TestCase) SkipPrecision() bool {
	switch tc.Operation {
	case "tosci", "toeng", "apply":
		return false
	default:
		return true
	}
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
			tc.Result = strings.ToUpper(cleanNumber(line[0]))
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

var GDAfiles = []string{
	"abs0",
	"add0",
	"base0",
	"compare0",
	"cuberoot-apd",
	"divide0",
	"divideint0",
	"exp0",
	"ln0",
	"log100",
	"minus0",
	"multiply0",
	"plus0",
	"power0",
	"quantize0",
	"randoms0",
	"randombound320",
	"reduce0",
	"remainder0",
	"rounding0",
	"squareroot0",
	"subtract0",
	"tointegral0",
}

func TestGDA(t *testing.T) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%10s%8s%8s%8s%8s%8s%8s\n", "name", "total", "success", "fail", "ignore", "skip", "missing")
	for _, fname := range GDAfiles {
		succeed := t.Run(fname, func(t *testing.T) {
			path, tcs := readGDA(t, fname)
			ignored, skipped, success, fail, total := gdaTest(t, path, tcs)
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

func (tc TestCase) Run(c *Context, done chan error, d, x, y *Decimal) (res Condition, err error) {
	switch tc.Operation {
	case "abs":
		res, err = c.Abs(d, x)
	case "add":
		res, err = c.Add(d, x, y)
	case "cuberoot":
		res, err = c.Cbrt(d, x)
	case "divide":
		res, err = c.Quo(d, x, y)
	case "divideint":
		res, err = c.QuoInteger(d, x, y)
	case "exp":
		res, err = c.Exp(d, x)
	case "ln":
		res, err = c.Ln(d, x)
	case "log10":
		res, err = c.Log10(d, x)
	case "minus":
		res, err = c.Neg(d, x)
	case "multiply":
		res, err = c.Mul(d, x, y)
	case "plus":
		res, err = c.Add(d, x, decimalZero)
	case "power":
		res, err = c.Pow(d, x, y)
	case "quantize":
		res, err = c.Quantize(d, x, y)
	case "reduce":
		d.Reduce(x)
	case "remainder":
		res, err = c.Rem(d, x, y)
	case "squareroot":
		res, err = c.Sqrt(d, x)
	case "subtract":
		res, err = c.Sub(d, x, y)
	case "tointegral":
		res, err = c.ToIntegral(d, x)
	default:
		done <- fmt.Errorf("unknown operation: %s", tc.Operation)
	}
	return
}

func BenchmarkGDA(b *testing.B) {
	for _, fname := range GDAfiles {
		b.Run(fname, func(b *testing.B) {
			_, tcs := readGDA(b, fname)
			res := new(Decimal)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
			Loop:
				for _, tc := range tcs {
					b.StopTimer()
					if GDAignore[tc.ID] || tc.Result == "?" || tc.HasNull() {
						continue
					}
					operands := make([]*Decimal, 2)
					for i, o := range tc.Operands {
						d, err := NewFromString(o)
						if err != nil {
							continue Loop
						}
						operands[i] = d
					}
					mode, ok := rounders[tc.Rounding]
					if !ok || mode == nil {
						b.Fatalf("unsupported rounding mode %s", tc.Rounding)
					}
					c := &Context{
						Precision:   uint32(tc.Precision),
						MaxExponent: int32(tc.MaxExponent),
						MinExponent: int32(tc.MinExponent),
						Rounding:    mode,
						Traps:       Subnormal | DefaultTraps,
					}
					b.StartTimer()
					_, err := tc.Run(c, nil, res, operands[0], operands[1])
					if err != nil {
						b.Fatalf("%s: %s", tc.ID, err)
					}
				}
			}
		})
	}
}

func readGDA(t testing.TB, name string) (string, []TestCase) {
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
	return path, tcs
}

func gdaTest(t *testing.T, path string, tcs []TestCase) (int, int, int, int, int) {
	var lock sync.Mutex
	var ignored, skipped, success, fail, total int
	for _, tc := range tcs {
		tc := tc
		succeed := t.Run(tc.ID, func(t *testing.T) {
			if *flagTime > 0 {
				timeDone := make(chan struct{}, 1)
				go func() {
					start := time.Now()
					for {
						select {
						case <-timeDone:
							return
						case <-time.After(*flagTime):
							fmt.Println(tc.ID, "running for", time.Since(start))
						}
					}
				}()
				defer func() { timeDone <- struct{}{} }()
			}
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
			case "toeng":
				t.Skip("unsupported")
			}
			if !*flagNoParallel && !*flagFailFast {
				t.Parallel()
			}
			// helpful acme address link
			t.Logf("%s:/^%s", path, tc.ID)
			t.Logf("%s %s = %s", tc.Operation, strings.Join(tc.Operands, " "), tc.Result)
			t.Logf("prec: %d, round: %s, Emax: %d, Emin: %d", tc.Precision, tc.Rounding, tc.MaxExponent, tc.MinExponent)
			mode, ok := rounders[tc.Rounding]
			if !ok || mode == nil {
				t.Fatalf("unsupported rounding mode %s", tc.Rounding)
			}
			operands := make([]*Decimal, 2)
			c := &Context{
				Precision:   uint32(tc.Precision),
				MaxExponent: int32(tc.MaxExponent),
				MinExponent: int32(tc.MinExponent),
				Rounding:    mode,
				Traps:       Subnormal | DefaultTraps,
			}
			var res, opres Condition
			for i, o := range tc.Operands {
				ctx := c
				if tc.SkipPrecision() {
					ctx = ctx.WithPrecision(0)
				}
				d, ores, err := c.NewFromString(o)
				if err != nil {
					testExponentError(t, err)
					if tc.Result == "?" {
						return
					}
					t.Fatalf("operand %d: %s: %+v", i, o, err)
				}
				operands[i] = d
				opres |= ores
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
				switch tc.Operation {
				case "compare":
					var c int
					c = operands[0].Cmp(operands[1])
					d.SetCoefficient(int64(c))
				case "tosci":
					s = operands[0].ToSci()
				default:
					res, err = tc.Run(c, done, d, operands[0], operands[1])
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
				if v.Cmp(operands[i]) != 0 {
					t.Fatalf("operand %d changed from %s to %s", i, o, operands[i])
				}
			}
			if !GDAignoreFlags[tc.ID] {
				var rcond Condition
				for _, cond := range tc.Conditions {
					switch cond {
					case "underflow":
						rcond |= Underflow
					case "inexact":
						rcond |= Inexact
					case "overflow":
						rcond |= Overflow
					case "subnormal":
						rcond |= Subnormal
					case "division_undefined":
						rcond |= DivisionUndefined
					case "division_by_zero":
						rcond |= DivisionByZero
					case "division_impossible":
						rcond |= DivisionImpossible
					case "invalid_operation":
						rcond |= InvalidOperation

					case "rounded":
						rcond |= Rounded
					case "lost_digits":
						// TODO(mjibson): implement this
					case "clamped", "invalid_context":
						// ignore

					default:
						t.Fatalf("unknown condition: %s", cond)
					}
				}

				// Add in the operand flags.
				res |= opres

				t.Logf("want flags (%d): %s", rcond, rcond)
				t.Logf("have flags (%d): %s", res, res)

				// TODO(mjibson): after upscaling, operations need to remove the 0s added
				// after the operation is done. Since this isn't happening, things are being
				// rounded when they shouldn't because the coefficient has so many trailing 0s.
				// Manually remove Rounded flag from context until the TODO is fixed.
				res &= ^Rounded
				rcond &= ^Rounded

				switch tc.Operation {
				case "log10", "power":
					// TODO(mjibson): Under certain conditions these are exact, but we don't
					// correctly mark them. Ignore these flags for now.
					// squareroot sometimes marks things exact when GDA says they should be
					// inexact.
					rcond &= ^Inexact
					res &= ^Inexact
				}

				// Don't worry about these flags; they are handled by GoError.
				res &= ^SystemOverflow
				res &= ^SystemUnderflow

				if (res.Overflow() || res.Underflow()) && (strings.HasPrefix(tc.ID, "rpow") ||
					strings.HasPrefix(tc.ID, "powr")) {
					t.Skip("overflow")
				}

				if rcond != res {
					t.Logf("got: %s (%#v)", d, d)
					t.Logf("error: %+v", err)
					t.Errorf("expected flags %q (%d); got flags %q (%d)", rcond, rcond, res, res)
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
			switch tc.Operation {
			case "tosci", "toeng":
				if s != tc.Result {
					t.Fatalf("expected %s, got %s", tc.Result, s)
				}
				return
			}
			r := newDecimal(t, testCtx, tc.Result)
			if d.Cmp(r) != 0 {
				// Some operations allow 1ulp of error in tests.
				switch tc.Operation {
				// TODO(mjibson): squareroot isn't supposed to allow 1ulp, but apparently
				// our implementation has some rounding errors.
				case "exp", "ln", "log10", "power":
					if d.Cmp(r) < 0 {
						d.Coeff.Add(&d.Coeff, bigOne)
					} else {
						r.Coeff.Add(&r.Coeff, bigOne)
					}
					if d.Cmp(r) == 0 {
						t.Logf("pass: within 1ulp: %s, %s", d, r)
						return
					}
				}
				if *flagPython {
					if tc.CheckPython(t, d) {
						return
					}
				}
				t.Logf("want: %s", tc.Result)
				t.Fatalf("got: %s (%#v)", d, d)
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
	switch tc.Operation {
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
	t.Logf("python script: %s", strings.Replace(script, "\n", "; ", -1))
	out, err := exec.Command("python", "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %s", err, out)
	}
	so := strings.TrimSpace(string(out))
	r := newDecimal(t, testCtx, so)
	c := d.Cmp(r)
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
	"radd163": true,
	"radd449": true,
	"sub062":  true,
	"sub063":  true,
	"sub067":  true,
	"sub068":  true,
	"sub080":  true,
	"sub142":  true,
	"sub143":  true,
	"sub332":  true,
	"sub333":  true,
	"sub342":  true,
	"sub343":  true,
	"sub363":  true,
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

	// GDA thinks these aren't subnormal, but python does
	"qua545": true,
	"qua547": true,
	"qua548": true,
	"qua549": true,

	// Invalid context errors, OK to skip.
	"ln901":  true,
	"ln903":  true,
	"ln905":  true,
	"log903": true,
	"log905": true,

	// Very large exponents we don't support yet
	"qua531": true,

	// TODO(mjibson): fix tests below

	// incorrect rounding
	"rpo213": true,
	"rpo412": true,

	// ln(x) at extreme input ranges
	"ln0901": true,
	"ln0902": true,

	// ln(x) of very small x, subnormals
	"ln759": true,
	"ln760": true,
	"ln761": true,
	"ln762": true,
	"ln763": true,
	"ln764": true,
	"ln765": true,
	"ln766": true,

	// log10(x) with large exponents, overflows
	"log0001": true,
	"log0020": true,
	"log1146": true,
	"log1147": true,
	"log1156": true,
	"log1157": true,
	"log1166": true,
	"log1167": true,

	// log10(x) where x is 1.0 +/- some tiny epsilon. Our results are close but
	// differ in the last places.
	"log1304": true,
	"log1309": true,
	"log1310": true,

	// The Vienna case
	"pow220": true,

	// very high precision
	"pow250": true,
	"pow251": true,
	"pow252": true,
	"pow253": true,
	"pow254": true,

	// x**y with very large y
	"pow063": true,
	"pow064": true,
	"pow065": true,
	"pow066": true,
	"pow118": true,
	"pow119": true,
	"pow120": true,
	"pow181": true,
	"pow182": true,
	"pow183": true,
	"pow184": true,
	"pow186": true,
	"pow187": true,
	"pow189": true,
	"pow190": true,
	"pow260": true,
	"pow261": true,
	"pow270": true,
	"pow271": true,
	"pow310": true,
	"pow311": true,
	"pow320": true,
	"pow321": true,
	"pow330": true,
	"pow331": true,
	"pow340": true,
	"pow341": true,

	// shouldn't overflow, but does
	"exp726":  true,
	"exp1236": true,
}

var GDAignoreFlags = map[string]bool{
	// unflagged overflow
	"exp705": true,

	// unflagged underflow
	"exp755": true,
	"exp760": true,
}
