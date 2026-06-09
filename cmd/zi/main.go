package main

import (
	"context"
	"fmt"
	"os"

	"zi/internal/zi"
)

func main() {
	env, err := zi.NewEnv()
	if err != nil {
		exit(1, err)
	}
	config, err := zi.NewConfig(env)
	if err != nil {
		exit(1, err)
	}
	runner := zi.NewRunner()
	git := zi.NewGit(env, config, runner)
	cache := zi.NewCache(git)
	prs := zi.NewPRService(git, runner)
	service := zi.NewService(env, config, git, cache, prs, runner)
	cli := zi.NewCLI(service, runner)

	code, err := cli.Run(context.Background(), os.Args[1:])
	if err != nil {
		exit(code, err)
	}
	os.Exit(code)
}

func exit(code int, err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}
