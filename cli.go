package cfft

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/alecthomas/kong"
	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
)

type CLI struct {
	Test    *TestCmd    `cmd:"" help:"test function"`
	Init    *InitCmd    `cmd:"" help:"initialize files"`
	Diff    *DiffCmd    `cmd:"" help:"diff function code"`
	Publish *PublishCmd `cmd:"" help:"publish function"`
	KVS     *KVSCmd     `cmd:"" help:"manage key-value store"`
	Render  *RenderCmd  `cmd:"" help:"render function code"`
	Util    *UtilCmd    `cmd:"" help:"utility commands"`
	TF      *TFCmd      `cmd:"tf" help:"output JSON for tf.json or external data source"`
	Version *VersionCmd `cmd:"" help:"show version"`

	Config    string `short:"c" long:"config" help:"config file" default:"cfft.yaml"`
	Debug     bool   `help:"enable debug log" default:"false"`
	LogFormat string `help:"log format (text,json)" default:"text" enum:"text,json"`
}

type TestCmd struct {
	CreateIfMissing bool   `help:"create function if missing" default:"false"`
	Run             string `help:"regexp to run test case names" default:""`

	runRegex *regexp.Regexp
	once     sync.Once
}

func (cmd *TestCmd) Setup() error {
	var err error
	cmd.once.Do(func() {
		if cmd.Run != "" {
			cmd.runRegex, err = regexp.Compile(cmd.Run)
			if err != nil {
				err = fmt.Errorf("failed to compile regexp %s, %w", cmd.Run, err)
			}
		}
	})
	return err
}

func (cmd *TestCmd) ShouldRun(name string) bool {
	if cmd.runRegex == nil {
		return true
	}
	return cmd.runRegex.MatchString(name)
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
	opt := &HandlerOptions{}
	opt.Level = logLevel
	if isatty.IsTerminal(os.Stderr.Fd()) {
		opt.Color = true
	}
	switch cli.LogFormat {
	case "text":
		slog.SetDefault(slog.New(NewLogHandler(os.Stderr, opt)))
	case "json":
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &opt.HandlerOptions)))
	default:
		return fmt.Errorf("invalid log format %s", cli.LogFormat)
	}
	if cli.Debug {
		logLevel.Set(slog.LevelDebug)
	} else {
		logLevel.Set(slog.LevelInfo)
	}

	var config *Config
	if cmds[0] != "init" && cmds[0] != "util" {
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
	if cmds[0] == "util" {
		// util commands don't need kvs
		return app.RunUtil(ctx, cmds[1], cli.Util)
	}

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
	case "render":
		return app.Render(ctx, cli.Render)
	case "kvs":
		return app.ManageKVS(ctx, cmds[1], cli.KVS)
	case "util":
		return app.RunUtil(ctx, cmds[1], cli.Util)
	case "tf":
		return app.RunTF(ctx, cli.TF)
	case "version":
		//
	default:
		panic("unknown command " + cmds[0])
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
