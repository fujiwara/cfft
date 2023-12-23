package cfft

import (
	"context"
	"fmt"
	"strings"

	"github.com/alecthomas/kong"
)

type CLI struct {
	Test    TestCmd    `cmd:"" help:"test function"`
	Init    InitCmd    `cmd:"" help:"initialize function"`
	Version VersionCmd `cmd:"" help:"show version"`

	Config string `short:"c" long:"config" help:"config file" default:"cfft.yaml"`
}

type TestCmd struct {
	CreateIfMissing bool `help:"create function if missing" default:"false"`
}

type InitCmd struct {
	Name      string `help:"function name" required:"true"`
	Format    string `help:"output event file format (json,jsonnet,yaml)" default:"json" enum:"jsonnet,json,yaml,yml"`
	EventType string `help:"event type (viewer-request,viewer-response)" default:"viewer-request" enum:"viewer-request,viewer-response"`
}

type VersionCmd struct{}

func RunCLI(ctx context.Context, args []string) error {
	var cli CLI
	parser, err := kong.New(&cli, kong.Vars{"version": Version})
	if err != nil {
		return err
	}
	kctx, err := parser.Parse(args)
	if err != nil {
		return err
	}
	cmd := strings.Fields(kctx.Command())[0]
	if cmd == "version" {
		fmt.Println("cfft version", Version)
		return nil
	}

	var config *Config
	if cmd != "init" {
		config, err = LoadConfig(ctx, cli.Config)
		if err != nil {
			return err
		}
	}
	app, err := New(ctx, config)
	if err != nil {
		return err
	}
	return app.Dispatch(ctx, cmd, &cli)
}

func (app *CFFT) Dispatch(ctx context.Context, cmd string, cli *CLI) error {
	switch cmd {
	case "test":
		return app.TestFunction(ctx, cli.Test)
	case "init":
		return app.InitFunction(ctx, cli.Init)
	case "version":
		//
	default:
		return nil
	}
	return nil
}
