package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"strings"

	"github.com/davidmdm/halloumi/internal"
	"github.com/davidmdm/halloumi/pkg/halloumi"
)

type MaydayParams struct {
	GlobalSettings
	Release string
}

//go:embed cmd_mayday_help.txt
var maydayHelp string

func init() {
	maydayHelp = strings.TrimSpace(internal.Colorize(maydayHelp))
}

func GetMaydayParams(settings GlobalSettings, args []string) (*MaydayParams, error) {
	flagset := flag.NewFlagSet("mayday", flag.ExitOnError)

	flagset.Usage = func() {
		fmt.Fprintln(flagset.Output(), maydayHelp)
		flagset.PrintDefaults()
	}

	params := MaydayParams{GlobalSettings: settings}

	RegisterGlobalFlags(flagset, &params.GlobalSettings)

	flagset.Parse(args)

	params.Release = flagset.Arg(0)
	if params.Release == "" {
		return nil, fmt.Errorf("release is required")
	}

	return &params, nil
}

func Mayday(ctx context.Context, params MaydayParams) error {
	client, err := halloumi.FromKubeConfig(params.KubeConfigPath)
	if err != nil {
		return fmt.Errorf("failed to instantiate k8 client: %w", err)
	}
	return client.Mayday(ctx, params.Release)
}
