package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/ko/pkg/build"
	"github.com/google/ko/pkg/publish"
)

type Options struct {
	ImageName  string
	ImportPath string
	BaseImage  string
	Push       bool
	Local      bool
	Tar        string
	Tags       tags
	Platform   string
}

type tags []string

func (t *tags) String() string {
	return fmt.Sprintf("%s", *t)
}

func (t *tags) Set(value string) error {
	*t = append(*t, value)
	return nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	err := run(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: sko <image> <path>\n")
	fmt.Fprintf(os.Stderr, "options:\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "examples:\n")
	fmt.Fprintf(os.Stderr, "\tsko dvob/http-server .\n")
	fmt.Fprintf(os.Stderr, "\tsko quay.io/foo/bar ./cmd/bar\n")
}

func run(ctx context.Context) error {
	opts := Options{}

	flag.Usage = usage
	flag.BoolVar(&opts.Push, "push", true, "Push image to registry.")
	flag.BoolVar(&opts.Local, "local", false, "Push image to local docker daemon.")
	flag.StringVar(&opts.Tar, "tar", "", "Save image to tar file.")
	flag.StringVar(&opts.BaseImage, "base", "gcr.io/distroless/static:nonroot", "Base image.")
	flag.StringVar(&opts.Platform, "platform", "linux/amd64", "Platform.")
	flag.Var(&opts.Tags, "tag", "Tags to publish. This option can be used multiple times. If not specified latest is used")

	flag.Parse()

	if flag.NArg() != 2 {
		usage()
		os.Exit(1)
	}

	opts.ImageName = flag.Arg(0)
	opts.ImportPath = flag.Arg(1)
	if len(opts.Tags) == 0 {
		opts.Tags = append(opts.Tags, "latest")
	}

	logs.Warn.SetOutput(os.Stderr)
	logs.Progress.SetOutput(os.Stderr)

	return buildAndPublish(ctx, opts)
}

func buildAndPublish(ctx context.Context, opts Options) error {
	baseImage, err := name.ParseReference(opts.BaseImage)
	if err != nil {
		return err
	}

	// platform := v1.Platform{
	// 	OS:           "linux",
	// 	Architecture: "amd64",
	// }

	remoteOpts := []remote.Option{
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
		remote.WithUserAgent("sko"),
		remote.WithContext(ctx),
		// remote.WithPlatform(platform),
	}

	buildOpts := []build.Option{
		build.WithBaseImages(func(_ context.Context, _ string) (build.Result, error) {
			// if desc.MediaType is Index/List and we want multiple platforms do return desc.ImageIndex()
			desc, err := remote.Get(baseImage, remoteOpts...)
			if err != nil {
				return nil, err
			}
			res, err := desc.Image()
			if err != nil {
				return nil, err
			}
			return res, nil
		}),
		// build.WithCreationTime(v1.Time{time.Now()}),
		build.WithDisabledOptimizations(),
		build.WithPlatforms(opts.Platform),
	}

	// add options here
	builder, err := build.NewGo(ctx, buildOpts...)
	if err != nil {
		return err
	}

	// builder = build.NewLimiter(builder, 20)
	builder, err = build.NewCaching(builder)
	if err != nil {
		return err
	}

	ignoreImportPathNamer := func(base, _ string) string {
		return base
		// return path.Join(base, path.Base(importPath))
	}

	publishers := []publish.Interface{}

	if opts.Tar != "" {
		publishers = append(publishers, publish.NewTarball(opts.Tar, opts.ImageName, ignoreImportPathNamer, opts.Tags))
	}

	if opts.Local {
		publishers = append(publishers, NewDaemon(ignoreImportPathNamer, opts.ImageName, opts.Tags))
	}

	if opts.Push {
		defaultPublisher, err := publish.NewDefault(opts.ImageName,
			publish.WithUserAgent("sko"),
			publish.WithAuthFromKeychain(authn.DefaultKeychain),
			publish.WithNamer(ignoreImportPathNamer),
			publish.WithTags(opts.Tags))
		if err != nil {
			return err
		}
		publishers = append(publishers, defaultPublisher)
	}

	publisher := publish.MultiPublisher(publishers...)
	publisher, err = publish.NewCaching(publisher)
	if err != nil {
		return err
	}
	defer publisher.Close()

	img, err := builder.Build(ctx, opts.ImportPath)
	if err != nil {
		return err
	}

	_, err = publisher.Publish(ctx, img, opts.ImportPath)
	if err != nil {
		return err
	}
	return nil
}
