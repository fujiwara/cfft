package cfft

import (
	"context"
	"fmt"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/fatih/color"
)

type CLI struct {
	Test    TestCmd    `cmd:"" help:"test function"`
	Init    InitCmd    `cmd:"" help:"initialize files"`
	Diff    DiffCmd    `cmd:"" help:"diff function code"`
	Publish PublishCmd `cmd:"" help:"publish function"`
	KVS     KVSCmd     `cmd:"" help:"manage key-value store"`
	Render  RenderCmd  `cmd:"" help:"render function code"`
	Version VersionCmd `cmd:"" help:"show version"`

	Config string `short:"c" long:"config" help:"config file" default:"cfft.yaml"`
}

type TestCmd struct {
	CreateIfMissing bool `help:"create function if missing" default:"false"`
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
	cmds := strings.Fields(kctx.Command())
	if cmds[0] == "version" {
		fmt.Println("cfft version", Version)
		return nil
	}

	var config *Config
	if cmds[0] != "init" {
		config, err = LoadConfig(ctx, cli.Config)
		if err != nil {
			return err
		}
	}
	app, err := New(ctx, config)
	if err != nil {
		return err
	}
	return app.Dispatch(ctx, cmds, &cli)
}

func (app *CFFT) Dispatch(ctx context.Context, cmds []string, cli *CLI) error {
	if err := app.prepareKVS(ctx, cli.Test.CreateIfMissing); err != nil {
		return err
	}

	for k, v := range app.envs {
		reset := localEnv(k, v)
		defer reset()
	}

	switch cmds[0] {
	case "test":
		return app.TestFunction(ctx, cli.Test)
	case "init":
		return app.InitFunction(ctx, cli.Init)
	case "diff":
		return app.DiffFunction(ctx, cli.Diff)
	case "publish":
		return app.PublishFunction(ctx, cli.Publish)
	case "kvs":
		return app.ManageKVS(ctx, cmds[1], cli.KVS)
	case "render":
		return app.Render(ctx, cli.Render)
	case "version":
		//
	default:
		return nil
	}
	return nil
}

func coloredDiff(src string) string {
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(line, "-") {
			b.WriteString(color.RedString(line) + "\n")
		} else if strings.HasPrefix(line, "+") {
			b.WriteString(color.GreenString(line) + "\n")
		} else {
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}
