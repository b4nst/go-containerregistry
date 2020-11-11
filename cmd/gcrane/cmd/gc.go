// Copyright 2018 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"log"
	"regexp"
	"time"

	"github.com/google/go-containerregistry/pkg/gcrane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/spf13/cobra"
)

// NewCmdGc creates a new cobra.Command for the gc subcommand.
func NewCmdGc() *cobra.Command {
	recursive := false
	before := time.Duration(0)
	pattern := ""

	cmd := &cobra.Command{
		Use:   "gc",
		Short: "List images that are not tagged",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			err := gc(args[0], recursive, time.Now().Add(-before), pattern)
			if err != nil {
				log.Fatalln(err)
			}
		},
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Whether to recurse through repos")
	cmd.Flags().DurationVarP(&before, "grace", "g", time.Duration(0), "Relative duration in which to ignore references. Only checks the uploaded time. If set, refs newer than the duration will not be print.")
	cmd.Flags().StringVarP(&pattern, "pattern", "p", "", "Will also collect images with no tags matching this pattern")

	return cmd
}

func gc(root string, recursive bool, before time.Time, pattern string) error {
	repo, err := name.NewRepository(root)
	if err != nil {
		return err
	}

	auth := google.WithAuthFromKeychain(gcrane.Keychain)

	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	filters := []func(manifest google.ManifestInfo) bool{
		func(manifest google.ManifestInfo) bool {
			return manifest.Uploaded.Before(before)
		},
		func(manifest google.ManifestInfo) bool {
			if pattern == "" {
				return len(manifest.Tags) == 0
			} else {
				for _, tag := range manifest.Tags {
					if re.MatchString(tag) {
						return false
					}
				}
				return true
			}
		},
	}

	printUntaggedImages := collector(filters)

	if recursive {
		if err := google.Walk(repo, printUntaggedImages, auth); err != nil {
			return err
		}
		return nil
	}

	tags, err := google.List(repo, auth)
	if err := printUntaggedImages(repo, tags, err); err != nil {
		return err
	}
	return nil
}

func collector(filters []func(manifest google.ManifestInfo) bool) func(repo name.Repository, tags *google.Tags, err error) error {
	return func(repo name.Repository, tags *google.Tags, err error) error {
		if err != nil {
			return err
		}

		for digest, manifest := range tags.Manifests {
			collect := true
			for _, f := range filters {
				if !f(manifest) {
					collect = false
					break
				}
			}
			if collect {
				fmt.Printf("%s@%s\n", repo, digest)
			}
		}

		return nil
	}
}
