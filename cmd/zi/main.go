package main

import (
	"context"
	"fmt"
	"os"

	"go.uber.org/fx"

	"zi/internal/zi"
)

func main() {
	code := 0

	app := fx.New(
		fx.NopLogger,
		fx.Provide(
			zi.NewEnv,
			zi.NewConfig,
			zi.NewRunner,
			zi.NewCache,
			zi.NewGit,
			zi.NewPRService,
			zi.NewService,
			zi.NewCLI,
		),
		fx.Invoke(func(lc fx.Lifecycle, cli *zi.CLI) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					var err error
					code, err = cli.Run(ctx, os.Args[1:])
					if err != nil {
						fmt.Fprintln(os.Stderr, err)
					}
					return err
				},
				OnStop: func(context.Context) error {
					return nil
				},
			})
		}),
	)

	if err := app.Start(context.Background()); err != nil {
		os.Exit(1)
	}
	if err := app.Stop(context.Background()); err != nil {
		os.Exit(1)
	}
	os.Exit(code)
}
