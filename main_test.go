// Copyright 2019 Matt Ho
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/savaki/ddb"
)

func TestCheckCoverage(t *testing.T) {
	var (
		ctx = context.Background()
		s   = session.Must(session.NewSession(aws.NewConfig().
			WithCredentials(credentials.NewStaticCredentials("blah", "blah", "")).
			WithEndpoint("http://localhost:8000").
			WithRegion("us-west-2")))
		api       = dynamodb.New(s)
		tableName = fmt.Sprintf("tmp-%v", time.Now().UnixNano())
		table     = ddb.New(api).MustTable(tableName, Record{})
	)

	err := table.CreateTableIfNotExists(ctx)
	if err != nil {
		t.Fatalf("got %v; want nil", err)
	}
	defer table.DeleteTableIfExists(ctx)

	testCases := map[string]struct {
		actual  float64
		min     float64
		last    float64
		wantErr bool
	}{
		"no check": {
			actual: 5,
			min:    0,
		},
		"first build - no min": {
			actual: 5,
			min:    0,
		},
		"first build": {
			actual: 5,
			min:    10,
		},
		"second build - ok": {
			actual: 5,
			min:    10,
			last:   1,
		},
		"second build - fail": {
			actual:  5,
			min:     10,
			last:    8,
			wantErr: true,
		},
	}

	for label, tc := range testCases {
		t.Run(label, func(t *testing.T) {
			opts := options{
				branch: label,
				commit: "blah",
				coverage: coverage{
					actual:  tc.actual,
					desired: tc.min,
				},
				repository: "blah",
				tableName:  tableName,
			}

			if tc.last > 0 {
				last := Record{
					Key:        makeKey(opts.repository, opts.branch),
					Number:     1,
					CommitHash: opts.commit,
					Coverage:   tc.last,
					CreatedAt:  time.Now().Format(time.RFC3339),
				}
				if err := table.Put(last).Run(); err != nil {
					t.Fatalf("got %v; want nil", err)
				}
			}

			err := checkCoverage(table, opts)
			if got, want := err != nil, tc.wantErr; got != want {
				t.Fatalf("got %v; want %v", got, want)
			}
		})
	}
}

func Test_findLast(t *testing.T) {
	var (
		ctx = context.Background()
		s   = session.Must(session.NewSession(aws.NewConfig().
			WithCredentials(credentials.NewStaticCredentials("blah", "blah", "")).
			WithEndpoint("http://localhost:8000").
			WithRegion("us-west-2")))
		api       = dynamodb.New(s)
		tableName = fmt.Sprintf("last-%v", time.Now().UnixNano())
		table     = ddb.New(api).MustTable(tableName, Record{})
		key       = "key"
	)

	err := table.CreateTableIfNotExists(ctx)
	if err != nil {
		t.Fatalf("got %v; want nil", err)
	}
	defer table.DeleteTableIfExists(ctx)

	a := Record{
		Key:       key,
		Number:    1,
		Coverage:  10,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	if err := table.Put(a).Run(); err != nil {
		t.Fatalf("got %v; want nil", err)
	}

	b := Record{
		Key:       key,
		Number:    2,
		Coverage:  20,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	if err := table.Put(b).Run(); err != nil {
		t.Fatalf("got %v; want nil", err)
	}

	last, err := findLast(table, key)
	if err != nil {
		t.Fatalf("got %v; want nil", err)
	}
	if got, want := last.Number, b.Number; got != want {
		t.Fatalf("got %v; want %v", got, want)
	}
}
