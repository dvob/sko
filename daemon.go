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

package main

// Copied from https://github.com/google/ko/blob/0b96f41240a4bcd515eb91af1e0b455b970447b6/pkg/publish/daemon.go
// Changed so that it supports other domain names than ko.local

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/ko/pkg/build"
	"github.com/google/ko/pkg/publish"
)

// demon is intentionally misspelled to avoid name collision (and drive Jon nuts).
type demon struct {
	namer    publish.Namer
	repoName string
	tags     []string
}

// NewDaemon returns a new publish.Interface that publishes images to a container daemon.
func NewDaemon(namer publish.Namer, repoName string, tags []string) publish.Interface {
	return &demon{namer, repoName, tags}
}

// Publish implements publish.Interface
func (d *demon) Publish(_ context.Context, br build.Result, s string) (name.Reference, error) {
	s = strings.TrimPrefix(s, build.StrictScheme)
	// https://github.com/google/go-containerregistry/issues/212
	s = strings.ToLower(s)

	// There's no way to write an index to a kind, so attempt to downcast it to an image.
	var img v1.Image
	switch i := br.(type) {
	case v1.Image:
		img = i
	case v1.ImageIndex:
		im, err := i.IndexManifest()
		if err != nil {
			return nil, err
		}
		goos, goarch := os.Getenv("GOOS"), os.Getenv("GOARCH")
		if goos == "" {
			goos = "linux"
		}
		if goarch == "" {
			goarch = "amd64"
		}
		for _, manifest := range im.Manifests {
			if manifest.Platform == nil {
				continue
			}
			if manifest.Platform.OS != goos {
				continue
			}
			if manifest.Platform.Architecture != goarch {
				continue
			}
			img, err = i.Image(manifest.Digest)
			if err != nil {
				return nil, err
			}
			break
		}
		if img == nil {
			return nil, fmt.Errorf("failed to find %s/%s image in index for image: %v", goos, goarch, s)
		}
	default:
		return nil, fmt.Errorf("failed to interpret %s result as image: %v", s, br)
	}

	digestTag, err := name.NewTag(fmt.Sprintf("%s:%s", d.namer(d.repoName, s), d.tags[0]))
	if err != nil {
		return nil, err
	}

	log.Printf("Loading %v", digestTag)
	if _, err := daemon.Write(digestTag, img); err != nil {
		return nil, err
	}
	log.Printf("Loaded %v", digestTag)

	for _, tagName := range d.tags {
		log.Printf("Adding tag %v", tagName)
		tag, err := name.NewTag(fmt.Sprintf("%s:%s", d.namer(d.repoName, s), tagName))
		if err != nil {
			return nil, err
		}

		err = daemon.Tag(digestTag, tag)

		if err != nil {
			return nil, err
		}
		log.Printf("Added tag %v", tagName)
	}

	return &digestTag, nil
}

func (d *demon) Close() error {
	return nil
}
