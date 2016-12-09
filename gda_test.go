package apd

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const testDir = "testdata"

type TestCase struct {
	Precision                int
	MaxExponent, MinExponent int
	Rounding                 string
	Extended, Clamp          bool

	Id         string
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
			tc.Id = line[0]
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

func TestGDAAdd(t *testing.T) {
	f, err := os.Open(filepath.Join(testDir, "add0.decTest"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tcs, err := ParseDecTest(f)
	if err != nil {
		t.Fatal(err)
	}
	ignore := map[string]bool{
		// weird rouding rules
		"add303": true,
		"add307": true,
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

		// large exponents
		"add607": true,
		"add617": true,
		"add627": true,
		"add637": true,
		"add647": true,
		"add657": true,
		"add667": true,
		"add677": true,
		"add687": true,
		"add697": true,
		"add707": true,
		"add717": true,
	}
	for _, tc := range tcs {
		t.Run(tc.Id, func(t *testing.T) {
			if ignore[tc.Id] {
				t.Skip("ignored")
			}
			if tc.HasNull() {
				t.Skip("has null")
			}
			mode, ok := rounders[tc.Rounding]
			if !ok {
				t.Fatalf("unsupported rounding mode %s", tc.Rounding)
			}
			a := newDecimal(t, tc.Operands[0])
			b := newDecimal(t, tc.Operands[1])
			d := new(Decimal)
			d.Precision = uint32(tc.Precision)
			d.MaxExponent = int32(tc.MaxExponent)
			d.MinExponent = int32(tc.MinExponent)
			d.Rounding = mode
			t.Logf("%s + %s = %s (prec: %d, round: %s)", tc.Operands[0], tc.Operands[1], tc.Result, tc.Precision, tc.Rounding)

			done := make(chan struct{}, 1)
			var err error
			go func() {
				_, err = d.Add(a, b)
				done <- struct{}{}
			}()
			select {
			case <-done:
			case <-time.After(time.Second / 4):
				t.Fatalf("timeout")
			}
			if tc.Result == "?" {
				if err != nil {
					return
				}
				t.Fatalf("expected error, got %#v", d)
			}
			r := newDecimal(t, tc.Result)
			c, err := d.Cmp(r)
			if err != nil {
				t.Fatal(err)
			}
			if c != 0 {
				t.Errorf("got: %#v", d)
			}
		})
	}
}
