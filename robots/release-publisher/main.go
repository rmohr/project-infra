/*
Copyright 2017 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"

	"github.com/google/go-github/github"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"k8s.io/test-infra/pkg/flagutil"
	prowflagutil "k8s.io/test-infra/prow/flagutil"
)

type options struct {
	port int

	dryRun bool
	github prowflagutil.GitHubOptions

	webhookSecretFile string
	mirrorRegex string
}

func (o *options) Validate() error {
	for _, group := range []flagutil.OptionGroup{&o.github} {
		if err := group.Validate(o.dryRun); err != nil {
			return err
		}
	}

	return nil
}

func gatherOptions() options {
	o := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.BoolVar(&o.dryRun, "dry-run", true, "Dry run for testing. Uses API tokens but does not mutate.")
	for _, group := range []flagutil.OptionGroup{&o.github} {
		group.AddFlags(fs)
	}
	fs.Parse(os.Args[1:])
	return o
}

func main() {
	o := gatherOptions()
	if err := o.Validate(); err != nil {
		logrus.Fatalf("Invalid options: %v", err)
	}

	logrus.SetFormatter(&logrus.JSONFormatter{})
	// TODO: Use global option from the prow config.
	logrus.SetLevel(logrus.DebugLevel)
	log := logrus.StandardLogger().WithField("robot", "release-publisher")

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: "... your access token ..."},
	)
	client := github.NewClient(oauth2.NewClient(ctx, ts))

	releases, _, err := client.Repositories.ListReleases(ctx,"kubevirt", "kubevirt", nil)
	if err != nil {
		log.Panicln(err)
	}

	semVerRegex := regexp.MustCompile(`^v([0-9]+)(\.[0-9]+)(\.[0-9]+)$`)
	validReleases := []*github.RepositoryRelease{}
	for _, release := range releases {
		if release.PublishedAt != nil {
			if !semVerRegex.MatchString(*release.TagName) {
				continue
			}
			validReleases = append(validReleases, release)
		}
	}

	sort.Slice(validReleases, func(i, j int) bool {
		return bytes.Compare([]byte(*validReleases[i].TagName), []byte(*validReleases[j].TagName)) < 0
	} )

	fmt.Println("latest stable release: " + *validReleases[len(validReleases)-1].TagName)
}
