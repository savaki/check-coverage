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
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/savaki/ddb"
	"github.com/urfave/cli"
)

type coverage struct {
	actual  float64
	desired float64
}

type options struct {
	branch     string
	commit     string
	coverage   coverage
	repository string
	tableName  string
}

var opts options

func main() {
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "b,branch",
			Usage:       "name of branch being built",
			Destination: &opts.branch,
		},
		cli.Float64Flag{
			Name:        "c,coverage",
			Usage:       "actual code coverage",
			Destination: &opts.coverage.actual,
		},
		cli.Float64Flag{
			Name:        "d,desired",
			Value:       90,
			Usage:       "minimum desired coverage; 90 == 90%",
			Destination: &opts.coverage.desired,
		},
		cli.StringFlag{
			Name:        "m,commit",
			Usage:       "commit hash",
			Destination: &opts.commit,
		},
		cli.StringFlag{
			Name:        "r,repository",
			Usage:       "name of repository",
			Destination: &opts.repository,
		},
		cli.StringFlag{
			Name:        "t,table",
			Usage:       "dynamodb table holding stats",
			Destination: &opts.tableName,
		},
	}
	app.Action = run
	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type Record struct {
	Key        string  `dynamodbav:"key"    ddb:"hash"`
	Number     int     `dynamodbav:"number" ddb:"range"`
	CommitHash string  `dynamodbav:"commit_hash,omitempty"`
	Coverage   float64 `dynamodbav:"coverage"`
	CreatedAt  string  `dynamodbav:"at"` // RFC3339
}

func makeKey(repo, branch string) string {
	return repo + ":" + branch
}

func findLast(table *ddb.Table, key string) (*Record, error) {
	query := table.Query("#Key = ?", key).
		ConsistentRead(true)

	var record Record
	if err := query.First(&record); err != nil {
		return nil, fmt.Errorf("unable to find record: %v", err)
	}

	return &record, nil
}

func run(_ *cli.Context) error {
	if opts.branch == "" {
		return fmt.Errorf("branch missing.  use --branch to specify branch name")
	}
	if opts.commit == "" {
		return fmt.Errorf("commit hash missing.  use --commit to specify the commit hash")
	}
	if opts.repository == "" {
		return fmt.Errorf("repository missing.  use --repository to specify branch name")
	}

	var (
		s      = session.Must(session.NewSession(aws.NewConfig()))
		api    = dynamodb.New(s)
		client = ddb.New(api)
		table  = client.MustTable(opts.tableName, Record{})
	)

	err := table.CreateTableIfNotExists(context.Background(), ddb.WithBillingMode(dynamodb.BillingModePayPerRequest))
	if err != nil {
		return err
	}

	return checkCoverage(table, opts)
}

func checkCoverage(table *ddb.Table, opts options) error {
	key := makeKey(opts.repository, opts.branch)

	last, err := findLast(table, key)
	if err != nil {
		return err
	}

	if opts.coverage.desired > 0 && opts.coverage.actual < last.Coverage {
		return fmt.Errorf("ERROR: build coverage targets not met.  build coverage, %.1f%%, below prior build coverage, %.1f%% (desired coverage: %.1f%%)", opts.coverage.actual, last.Coverage, opts.coverage.desired)
	}

	record := Record{
		Key:       key,
		Number:    last.Number + 1,
		Coverage:  opts.coverage.actual,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	put := table.Put(record).
		Condition("attribute_not_exists(#Number)")
	if err := put.Run(); err != nil {
		return fmt.Errorf("unable to save coverage record: %v", err)
	}

	return nil
}
