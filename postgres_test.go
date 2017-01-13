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

// +build postgres

// Run a test against Postgres (or CockroachDB) servers:
// go test -run Postgres -tags postgres -postgres 'user=postgres sslmode=disable;postgresql://root@localhost:26277?sslmode=disable'

package apd

import (
	crand "crypto/rand"
	"database/sql"
	"encoding/binary"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"strings"
	"testing"

	_ "github.com/lib/pq"
	"github.com/pkg/errors"
)

var (
	flagPostgres = flag.String("postgres", "postgres://postgres@localhost?sslmode=disable", "Postgres connection strings; specify multiple with semicolons")
)

func TestPostgres(t *testing.T) {
	var seed int64
	err := binary.Read(crand.Reader, binary.LittleEndian, &seed)
	if err != nil {
		t.Fatal(err)
	}
	rnd := rand.New(rand.NewSource(seed))

	cs := strings.Split(*flagPostgres, ";")
	conns := make([]*sql.DB, len(cs))
	for i, s := range cs {
		conn, err := sql.Open("postgres", s)
		if err != nil {
			t.Fatalf("%s: %s", s, err)
		}
		conns[i] = conn
	}

	slots := make(chan struct{}, runtime.GOMAXPROCS(0))

	errch := make(chan error)
	go func() {
		for {
			slots <- struct{}{}
			go func() {
				defer func() { <-slots }()
				f := Float64(rnd)
				ds := fmt.Sprint(f)
				y := Float64(rnd)
				dy := fmt.Sprint(y)
				q := fmt.Sprintf("SELECT (%s::decimal / %s::decimal)::text;", ds, dy)
				res := make([]string, len(conns))
				digits := make([]int64, len(conns))
				for i, db := range conns {
					var s string
					if err := db.QueryRow(q).Scan(&s); err != nil {
						return
						errch <- errors.Errorf("%s: %s: %s", cs[i], q, err)
					}
					d, err := NewFromString(s)
					if err != nil {
						errch <- errors.Errorf("%s: %s", s, err)
					}
					res[i] = d.ToStandard()
					digits[i] = d.NumDigits()
				}
				for i, r := range res {
					c := BaseContext.WithPrecision(uint32(digits[i]))
					d, err := NewFromString(ds)
					if err != nil {
						errch <- errors.Errorf("%s: %s", ds, err)
					}
					y, err := NewFromString(dy)
					if err != nil {
						errch <- errors.Errorf("%s: %s", dy, err)
					}
					if _, err := c.Quo(d, d, y); err != nil {
						errch <- errors.Errorf("%s: %s", d, err)
					}
					rs := d.ToStandard()
					sd := sameDigits(rs, r)
					info := errors.Errorf("%s\n\tdigits: %v, sd: %v\n\t%v (apd)\n\t%v (%s)\n", q, digits[i], sd, rs, r, cs[i])
					if rs != r {
						errch <- info
					} else {
						fmt.Println(info)
					}
				}
			}()
		}
	}()
	t.Fatalf("%+v", <-errch)
}

// sameDigits returns the number of identical digits of a and b.
func sameDigits(a, b string) int {
	s := 0
	m := 0
	for s < len(a) && s < len(b) && a[s] == b[s] {
		switch a[s] {
		case '-', '.':
			m++
		}
		s++
	}
	return s - m
}

func Float64(rand *rand.Rand) float64 {
	v := rand.Float64()
	switch rand.Intn(3) {
	case 0:
		i := rand.Intn(75)
		v *= math.Pow10(i)
	case 1:
		i := rand.Intn(75)
		v *= math.Pow10(-i)
	case 2:
		// ignore
	default:
		panic("nope")
	}
	return v
}
