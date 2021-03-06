//go:generate go-bindata -pkg main -o credits.go vendor/CREDITS
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/pkg/errors"

	"github.com/yuuki/binrep/pkg/command"
	"github.com/yuuki/binrep/pkg/config"
)

const (
	defaultKeepReleases int = 5
)

var (
	creditsText = string(MustAsset("vendor/CREDITS"))
)

// CLI is the command line object.
type CLI struct {
	// outStream and errStream are the stdout and stderr
	// to write message from the CLI.
	outStream, errStream io.Writer
}

func main() {
	cli := &CLI{outStream: os.Stdout, errStream: os.Stderr}
	os.Exit(cli.Run(os.Args))
}

// Run invokes the CLI with the given arguments.
func (cli *CLI) Run(args []string) int {
	if len(args) <= 1 {
		fmt.Fprint(cli.errStream, helpText)
		return 2
	}

	config.Load()

	var err error
	i := 1
ARG_LOOP:
	for i < len(args) {
		switch cmd := args[i]; cmd {
		case "list":
			err = cli.doList(args[i+1:])
			break ARG_LOOP
		case "show":
			err = cli.doShow(args[i+1:])
			break ARG_LOOP
		case "push":
			err = cli.doPush(args[i+1:])
			break ARG_LOOP
		case "pull":
			err = cli.doPull(args[i+1:])
			break ARG_LOOP
		case "--version":
			fmt.Fprintf(cli.errStream, "%s version %s, build %s, date %s \n", name, version, commit, date)
			return 0
		case "--credits":
			fmt.Fprintln(cli.outStream, creditsText)
			return 0
		case "-h", "--help":
			fmt.Fprint(cli.errStream, helpText)
			return 0
		case "-e", "--endpoint":
			if len(args) <= i+1 {
				fmt.Fprint(cli.errStream, "want --endpoint value")
				fmt.Fprint(cli.errStream, helpText)
				return 1
			}
			config.Config.BackendEndpoint = args[i+1]
			i += 2
			// No subcommand error
			if len(args) <= i {
				fmt.Fprint(cli.errStream, helpText)
				return 1
			}
		default:
			fmt.Fprintf(cli.errStream, "%s is undefined subcommand or option\n", cmd)
			fmt.Fprint(cli.errStream, helpText)
			return 1
		}
	}

	if err != nil {
		fmt.Fprintln(cli.errStream, err)
		return 2
	}

	return 0
}

var helpText = `Usage: binrep [options]

  The static binary repository manager.

Commands:
  list		show releases on remote repository
  show          show binary information.
  push		push binary.
  pull		pull binary.

Options:
  --version             print version
  --help, -h            print help
`

func validateConfig() error {
	if config.Config.BackendEndpoint == "" {
		return errors.New("BackendEndpoint required. Use --endpoint or BINREP_BACKEND_ENDPOINT")
	}
	return nil
}

func (cli *CLI) prepareFlags(help string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(cli.errStream)
	flags.Usage = func() {
		fmt.Fprint(cli.errStream, help)
	}
	return flags
}

var listHelpText = `Usage: binrep list [options]

show releases on remote repository

Options:
`

func (cli *CLI) doList(args []string) error {
	var param command.ListParam
	flags := cli.prepareFlags(listHelpText)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if len(flags.Args()) != 0 {
		fmt.Fprint(cli.errStream, listHelpText)
		return errors.Errorf("extra arguments")
	}
	if err := validateConfig(); err != nil {
		return err
	}
	return command.List(&param)
}

var showHelpText = `Usage: binrep show [options] <host>/<user>/<project>

show binary information.

Options:
  --timestamp, -t       binary timestamp
`

func (cli *CLI) doShow(args []string) error {
	var param command.ShowParam
	flags := cli.prepareFlags(showHelpText)
	flags.StringVar(&param.Timestamp, "t", "", "")
	flags.StringVar(&param.Timestamp, "timestamp", "", "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if len(flags.Args()) < 1 {
		fmt.Fprint(cli.errStream, showHelpText)
		return errors.Errorf("too few arguments")
	}
	if err := validateConfig(); err != nil {
		return err
	}
	return command.Show(&param, flags.Arg(0))
}

var pushHelpText = `Usage: binrep push [options] <host>/<user>/<project> /path/to/binary ...

push binary.

Options:
  --timestamp, -t       binary timestamp
  --keep-releases, -k	the number of releases that it keeps (default: 5)
  --force, -f		always push even if each checksum of binaries is the same with each one on remote storage (default: false)
`

func (cli *CLI) doPush(args []string) error {
	var param command.PushParam
	flags := cli.prepareFlags(pushHelpText)
	flags.StringVar(&param.Timestamp, "t", "", "")
	flags.StringVar(&param.Timestamp, "timestamp", "", "")
	flags.IntVar(&param.KeepReleases, "k", defaultKeepReleases, "")
	flags.IntVar(&param.KeepReleases, "keep-releases", defaultKeepReleases, "")
	flags.BoolVar(&param.Force, "f", false, "")
	flags.BoolVar(&param.Force, "force", false, "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	argLen := len(flags.Args())
	if argLen < 2 {
		fmt.Fprint(cli.errStream, pushHelpText)
		return errors.Errorf("too few arguments")
	}
	if err := validateConfig(); err != nil {
		return err
	}
	return command.Push(&param, flags.Arg(0), flags.Args()[1:argLen])
}

var pullHelpText = `Usage: binrep pull [options] <host>/<user>/<project> /path/to/binary

pull binary.

Options:
  --timestamp, -t       binary timestamp
  --max-bandwidth, -bw	max bandwidth for download binaries (Bytes/sec) eg. '1 MB', '1024 KB'
`

func (cli *CLI) doPull(args []string) error {
	var param command.PullParam
	flags := cli.prepareFlags(pullHelpText)
	flags.StringVar(&param.Timestamp, "t", "", "")
	flags.StringVar(&param.Timestamp, "timestamp", "", "")
	flags.StringVar(&param.MaxBandWidth, "bw", "", "")
	flags.StringVar(&param.MaxBandWidth, "max-bandwidth", "", "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if len(flags.Args()) != 2 {
		fmt.Fprint(cli.errStream, pullHelpText)
		return errors.Errorf("too few or many arguments")
	}
	if err := validateConfig(); err != nil {
		return err
	}
	return command.Pull(&param, flags.Arg(0), flags.Arg(1))
}
