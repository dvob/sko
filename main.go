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
	// Load into local docker daemon and to not push to remote registry
	Local    bool
	Tar      string
	Tags     tags
	Platform string
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
	fmt.Fprintf(os.Stderr, "\tsko -tag v0.0.4 quay.io/foo/bar ./cmd/bar\n")
}

func run(ctx context.Context) error {
	opts := Options{
		BaseImage: "gcr.io/distroless/static:nonroot",
		Platform:  "linux/amd64",
		Push:      true,
	}

	flag.Usage = usage
	flag.BoolVar(&opts.Local, "local", opts.Local, "Load image into local docker daemon and do not push to Docker registry.")
	flag.StringVar(&opts.Tar, "tar", "", "Save image to tar file instead of pushing it somewhere.")
	flag.StringVar(&opts.BaseImage, "base", opts.BaseImage, "Base image.")
	flag.StringVar(&opts.Platform, "platform", opts.Platform, "Platform.")
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

	remoteOpts := []remote.Option{
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
		remote.WithUserAgent("sko"),
		remote.WithContext(ctx),
	}

	buildOpts := []build.Option{
		build.WithBaseImages(func(_ context.Context, _ string) (build.Result, error) {
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
		build.WithDisabledOptimizations(),
		build.WithPlatforms(opts.Platform),
	}

	builder, err := build.NewGo(ctx, buildOpts...)
	if err != nil {
		return err
	}

	builder, err = build.NewCaching(builder)
	if err != nil {
		return err
	}

	ignoreImportPathNamer := func(base, _ string) string {
		return base
	}

	publishers := []publish.Interface{}

	if opts.Tar != "" {
		publishers = append(publishers, publish.NewTarball(opts.Tar, opts.ImageName, ignoreImportPathNamer, opts.Tags))
	} else if opts.Local {
		publishers = append(publishers, NewDaemon(ignoreImportPathNamer, opts.ImageName, opts.Tags))
	} else {
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
