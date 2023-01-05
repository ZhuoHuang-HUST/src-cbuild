package container

import (
	"fmt"
	"io"

	"golang.org/x/net/context"

//	"github.com/docker/docker/api/types/versions"
//	"github.com/docker/docker/cli/command/commands"
//	cliflags "github.com/docker/docker/cli/flags"
//	"github.com/docker/docker/cliconfig"
//	"github.com/docker/docker/dockerversion"
//	"github.com/docker/docker/pkg/term"
//  "github.com/docker/docker/utils"
//	"github.com/spf13/pflag"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	apiclient "github.com/docker/docker/client"
	options "github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/promise"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/spf13/cobra"
)

type execOptions struct {
	detachKeys  string
	interactive bool
	tty         bool
	detach      bool
	user        string
	privileged  bool
	env         *options.ListOpts
}

func newExecOptions() *execOptions {
	var values []string
	return &execOptions{
		env: options.NewListOptsRef(&values, runconfigopts.ValidateEnv),
	}
}

// NewExecCommand creats a new cobra.Command for `docker exec`
func NewExecCommand(dockerCli *command.DockerCli) *cobra.Command {
    fmt.Println("cli/command/container/exec.go  NewExecCommand()")

	opts := newExecOptions()

	cmd := &cobra.Command{
		Use:   "exec [OPTIONS] CONTAINER COMMAND [ARG...]",
		Short: "Run a command in a running container",
		Args:  cli.RequiresMinArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			container := args[0]
			execCmd := args[1:]
            fmt.Println("cli/command/container/exec.go  NewExecCommand() container : ", container)
            fmt.Println("cli/command/container/exec.go  NewExecCommand() execCmd : ", execCmd)
            fmt.Println("cli/command/container/exec.go  NewExecCommand() before runExec()")
			return runExec(dockerCli, opts, container, execCmd)
		},
	}
    
    fmt.Println("cli/command/container/exec.go  NewExecCommand() after runExec()")

	flags := cmd.Flags()
	flags.SetInterspersed(false)

	flags.StringVarP(&opts.detachKeys, "detach-keys", "", "", "Override the key sequence for detaching a container")
	flags.BoolVarP(&opts.interactive, "interactive", "i", false, "Keep STDIN open even if not attached")
	flags.BoolVarP(&opts.tty, "tty", "t", false, "Allocate a pseudo-TTY")
	flags.BoolVarP(&opts.detach, "detach", "d", false, "Detached mode: run command in the background")
	flags.StringVarP(&opts.user, "user", "u", "", "Username or UID (format: <name|uid>[:<group|gid>])")
	flags.BoolVarP(&opts.privileged, "privileged", "", false, "Give extended privileges to the command")
	flags.VarP(opts.env, "env", "e", "Set environment variables")
	flags.SetAnnotation("env", "version", []string{"1.25"})

	return cmd
}

func runExec(dockerCli *command.DockerCli, opts *execOptions, container string, execCmd []string) error {
	fmt.Println("cli/command/container/exec.go  runExec()")
    
    execConfig, err := parseExec(opts, execCmd)
	// just in case the ParseExec does not exit
	if container == "" || err != nil {
		return cli.StatusError{StatusCode: 1}
	}

	if opts.detachKeys != "" {
		fmt.Println("cli/command/container/exec.go  runExec() opts.detachKeys!=null")
        dockerCli.ConfigFile().DetachKeys = opts.detachKeys
	}

	// Send client escape keys
	execConfig.DetachKeys = dockerCli.ConfigFile().DetachKeys

	ctx := context.Background()
	client := dockerCli.Client()
	fmt.Println("cli/command/container/exec.go  runExec() Client : ", client)
	fmt.Println("cli/command/container/exec.go  runExec() ClientVersion : ", client.ClientVersion())

	response, err := client.ContainerExecCreate(ctx, container, *execConfig)
	if err != nil {
		return err
	}

	execID := response.ID
	if execID == "" {
		fmt.Fprintf(dockerCli.Out(), "exec ID empty")
		return nil
	}
    
    fmt.Println("cli/command/container/exec.go  runExec() execConfig.Detach : ", execConfig.Detach)
	//Temp struct for execStart so that we don't need to transfer all the execConfig
	if !execConfig.Detach {
		if err := dockerCli.In().CheckTty(execConfig.AttachStdin, execConfig.Tty); err != nil {
			return err
		}
	} else {
		execStartCheck := types.ExecStartCheck{
			Detach: execConfig.Detach,
			Tty:    execConfig.Tty,
        }

      	fmt.Println("cli/command/container/exec.go  runExec() ctx : ", ctx)
	    fmt.Println("cli/command/container/exec.go  runExec() execConfig : ", execConfig)
     	fmt.Println("cli/command/container/exec.go  runExec() execStartCheck : ", execStartCheck)
	    fmt.Println("cli/command/container/exec.go  runExec() client : ", client)
	    fmt.Println("cli/command/container/exec.go  runExec() response : ", response)

		if err := client.ContainerExecStart(ctx, execID, execStartCheck); err != nil {
			return err
		}
		// For now don't print this - wait for when we support exec wait()
		// fmt.Fprintf(dockerCli.Out(), "%s\n", execID)
		return nil
	}

	// Interactive exec requested.
	var (
		out, stderr io.Writer
		in          io.ReadCloser
		errCh       chan error
	)

	if execConfig.AttachStdin {
		in = dockerCli.In()
	}
	if execConfig.AttachStdout {
		out = dockerCli.Out()
	}
	if execConfig.AttachStderr {
		if execConfig.Tty {
			stderr = dockerCli.Out()
		} else {
			stderr = dockerCli.Err()
		}
	}

	resp, err := client.ContainerExecAttach(ctx, execID, *execConfig)
	if err != nil {
		return err
	}
	defer resp.Close()
	errCh = promise.Go(func() error {
		return holdHijackedConnection(ctx, dockerCli, execConfig.Tty, in, out, stderr, resp)
	})

	if execConfig.Tty && dockerCli.In().IsTerminal() {
		if err := MonitorTtySize(ctx, dockerCli, execID, true); err != nil {
			fmt.Fprintf(dockerCli.Err(), "Error monitoring TTY size: %s\n", err)
		}
	}

	if err := <-errCh; err != nil {
		logrus.Debugf("Error hijack: %s", err)
		return err
	}

	var status int
	if _, status, err = getExecExitCode(ctx, client, execID); err != nil {
		return err
	}

	if status != 0 {
		return cli.StatusError{StatusCode: status}
	}

	return nil
}



func RunExecInFirstContainer(dockerCli *command.DockerCli) error {
	fmt.Println("cli/command/container/exec.go  RunExecInFirstContainer()")

    //execConfig, err := parseExec(opts, execCmd)
	// just in case the ParseExec does not exit
	//if container == "" || err != nil {
	//	return cli.StatusError{StatusCode: 1}
	//}

    opts := newExecOptions()
	if opts.detachKeys != "" {
		dockerCli.ConfigFile().DetachKeys = opts.detachKeys
	}

	// Send client escape keys
    execConfig := dockerCli.GetCliexecconfig()
//	execConfig.DetachKeys = dockerCli.ConfigFile().DetachKeys

    container := dockerCli.GetClicontainer()

	ctx := context.Background()

/*    var flags *pflag.FlagSet
    cliopts := cliflags.NewClientOptions()
    if dockerCli.Client() == nil {
       fmt.Println("cli/command/container/exec.go  RunExecInFirstContainer() dockerCli.Client() is null!!!")
       cliopts.Common.SetDefaultOptions(flags)
       dockerPreRun(cliopts)
       if err := dockerCli.Initialize(cliopts); err != nil {
          fmt.Println("cli/command/container/exec.go  RunExecInFirstContainer() dockerCli.Initialize is err!!!")
       }
   }
*/

//    client := dockerCli.Client()
    tmpclient, err := apiclient.NewEnvClient()
    if err != nil {
       fmt.Println("cli/command/container/exec.go  RunExecInFirstContainer() NewEnvClient() is err!!!")
    }
    headers := map[string]string{"User-Agent":"Docker-Client/1.14.0-dev (linux)"}
    tmpclient.SetCustomHTTPHeaders(headers)
    if error := dockerCli.SetCliclient(tmpclient); error != nil {
       fmt.Println("cli/command/container/exec.go  RunExecInFirstContainer() dockerCli.SetClient is err!!!")
    }

    client := dockerCli.Client()
    fmt.Println("cli/command/container/exec.go  RunExecInFirstContainer() NewEnvClient() client : ", client)

/*		client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsc,
			},
		}
	}

	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		host = DefaultDockerHost
	}
	version := os.Getenv("DOCKER_API_VERSION")
	if version == "" {
		version = DefaultVersion
	}

	cli, err := NewClient(host, version, client, nil)
*/
    fmt.Println("cli/command/container/exec.go  RunExecInFirstContainer() *execConfig : ", execConfig)
	response, err := client.ContainerExecCreate(ctx, container, *execConfig)
	if err != nil {
		return err
	}

	execID := response.ID
	if execID == "" {
		fmt.Fprintf(dockerCli.Out(), "exec ID empty")
		return nil
	}
    fmt.Println("cli/command/container/exec.go  RunExecInFirstContainer() after ContainerExecCreate()")
    
    fmt.Println("cli/command/container/exec.go  RunExecInFirstContainer() execConfig.Detach : ", execConfig.Detach)
	//Temp struct for execStart so that we don't need to transfer all the execConfig
	if !execConfig.Detach {
		if err := dockerCli.In().CheckTty(execConfig.AttachStdin, execConfig.Tty); err != nil {
			return err
		}
	} else {
		execStartCheck := types.ExecStartCheck{
			Detach: execConfig.Detach,
			Tty:    execConfig.Tty,
		}

        fmt.Println("cli/command/container/exec.go  RunExecInFirstContainer() execStarCheck() : ", execStartCheck)
        fmt.Println("cli/command/container/exec.go  RunExecInFirstContainer() response : ", response)
        fmt.Println("cli/command/container/exec.go  RunExecInFirstContainer() ctx : ", ctx)

        fmt.Println("cli/command/container/exec.go  RunExecInFirstContainer() before ContainerExecStart()")
		if err := client.ContainerExecStart(ctx, execID, execStartCheck); err != nil {
            fmt.Println("cli/command/container/exec.go  RunExecInFirstContainer() ContainerExecStart()i is err!!!!")
			return err
		}
		// For now don't print this - wait for when we support exec wait()
		// fmt.Fprintf(dockerCli.Out(), "%s\n", execID)
		return nil
	}
    fmt.Println("cli/command/container/exec.go  RunExecInFirstContainer() after ContainerExecStart()")

	// Interactive exec requested.
	var (
		out, stderr io.Writer
		in          io.ReadCloser
		errCh       chan error
	)

	if execConfig.AttachStdin {
		in = dockerCli.In()
	}
	if execConfig.AttachStdout {
		out = dockerCli.Out()
	}
	if execConfig.AttachStderr {
		if execConfig.Tty {
			stderr = dockerCli.Out()
		} else {
			stderr = dockerCli.Err()
		}
	}

	resp, err := client.ContainerExecAttach(ctx, execID, *execConfig)
	if err != nil {
		return err
	}
	defer resp.Close()
	errCh = promise.Go(func() error {
		return holdHijackedConnection(ctx, dockerCli, execConfig.Tty, in, out, stderr, resp)
	})

	if execConfig.Tty && dockerCli.In().IsTerminal() {
		if err := MonitorTtySize(ctx, dockerCli, execID, true); err != nil {
			fmt.Fprintf(dockerCli.Err(), "Error monitoring TTY size: %s\n", err)
		}
	}

	if err := <-errCh; err != nil {
		logrus.Debugf("Error hijack: %s", err)
		return err
	}

	var status int
	if _, status, err = getExecExitCode(ctx, client, execID); err != nil {
		return err
	}

	if status != 0 {
		return cli.StatusError{StatusCode: status}
	}
    
    fmt.Println("cli/command/container/exec.go  RunExecInFirstContainer() end")

	return nil
}







// getExecExitCode perform an inspect on the exec command. It returns
// the running state and the exit code.
func getExecExitCode(ctx context.Context, client apiclient.ContainerAPIClient, execID string) (bool, int, error) {
	resp, err := client.ContainerExecInspect(ctx, execID)
	if err != nil {
		// If we can't connect, then the daemon probably died.
		if !apiclient.IsErrConnectionFailed(err) {
			return false, -1, err
		}
		return false, -1, nil
	}

	return resp.Running, resp.ExitCode, nil
}

// parseExec parses the specified args for the specified command and generates
// an ExecConfig from it.
func parseExec(opts *execOptions, execCmd []string) (*types.ExecConfig, error) {
	execConfig := &types.ExecConfig{
		User:       opts.user,
		Privileged: opts.privileged,
		Tty:        opts.tty,
		Cmd:        execCmd,
		Detach:     opts.detach,
	}

	// If -d is not set, attach to everything by default
	if !opts.detach {
		execConfig.AttachStdout = true
		execConfig.AttachStderr = true
		if opts.interactive {
			execConfig.AttachStdin = true
		}
	}

	if opts.env != nil {
		execConfig.Env = opts.env.GetAll()
	}

	return execConfig, nil
}
















/*
func newFirstDockerCommand(dockerCli *command.DockerCli) *cobra.Command {
    fmt.Println("cli/command/container/exec.go  newFirstDockerCommand()")
	opts := cliflags.NewClientOptions()
	var flags *pflag.FlagSet

	cmd := &cobra.Command{
		Use:              "docker [OPTIONS] COMMAND [ARG...]",
		Short:            "A self-sufficient runtime for containers",
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
		Args:             noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Version {
				showVersion()
				return nil
			}
			return dockerCli.ShowHelp(cmd, args)
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// daemon command is special, we redirect directly to another binary
			if cmd.Name() == "daemon" {
				return nil
			}
			// flags must be the top-level command flags, not cmd.Flags()
			opts.Common.SetDefaultOptions(flags)
			dockerPreRun(opts)
			if err := dockerCli.Initialize(opts); err != nil {
				return err
			}
			return isSupported(cmd, dockerCli.Client().ClientVersion(), dockerCli.HasExperimental())
		},
	}
	cli.SetupRootCommand(cmd)

	cmd.SetHelpFunc(func(ccmd *cobra.Command, args []string) {
		if dockerCli.Client() == nil { // when using --help, PersistenPreRun is not called, so initialization is needed.
			// flags must be the top-level command flags, not cmd.Flags()
			opts.Common.SetDefaultOptions(flags)
			dockerPreRun(opts)
			dockerCli.Initialize(opts)
		}

		if err := isSupported(ccmd, dockerCli.Client().ClientVersion(), dockerCli.HasExperimental()); err != nil {
			ccmd.Println(err)
			return
		}

		hideUnsupportedFeatures(ccmd, dockerCli.Client().ClientVersion(), dockerCli.HasExperimental())

		if err := ccmd.Help(); err != nil {
			ccmd.Println(err)
		}
	})

	flags = cmd.Flags()
	flags.BoolVarP(&opts.Version, "version", "v", false, "Print version information and quit")
	flags.StringVar(&opts.ConfigDir, "config", cliconfig.ConfigDir(), "Location of client config files")
	opts.Common.InstallFlags(flags)

	cmd.SetOutput(dockerCli.Out())
	cmd.AddCommand(newDaemonCommand())
	commands.AddCommands(cmd, dockerCli)
    
    fmt.Println("cli/command/container/exec.go  newFirstDockerCommand() end ")

	return cmd
}

func noArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
    fmt.Println("cli/command/container/exec.go noArgs() err : is not a docker cmmand")
	return fmt.Errorf(
		"docker: '%s' is not a docker command.\nSee 'docker --help'", args[0])
}


func showVersion() {
	fmt.Printf("Docker version %s, build %s\n", dockerversion.Version, dockerversion.GitCommit)
}

func dockerPreRun(opts *cliflags.ClientOptions) {
	cliflags.SetLogLevel(opts.Common.LogLevel)

	if opts.ConfigDir != "" {
		cliconfig.SetConfigDir(opts.ConfigDir)
	}

	if opts.Common.Debug {
		utils.EnableDebug()
	}
}

func hideUnsupportedFeatures(cmd *cobra.Command, clientVersion string, hasExperimental bool) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// hide experimental flags
		if !hasExperimental {
			if _, ok := f.Annotations["experimental"]; ok {
				f.Hidden = true
			}
		}

		// hide flags not supported by the server
		if flagVersion, ok := f.Annotations["version"]; ok && len(flagVersion) == 1 && versions.LessThan(clientVersion, flagVersion[0]) {
			f.Hidden = true
		}

	})

	for _, subcmd := range cmd.Commands() {
		// hide experimental subcommands
		if !hasExperimental {
			if _, ok := subcmd.Tags["experimental"]; ok {
				subcmd.Hidden = true
			}
		}

		// hide subcommands not supported by the server
		if subcmdVersion, ok := subcmd.Tags["version"]; ok && versions.LessThan(clientVersion, subcmdVersion) {
			subcmd.Hidden = true
		}
	}
}

func isSupported(cmd *cobra.Command, clientVersion string, hasExperimental bool) error {
	if !hasExperimental {
		if _, ok := cmd.Tags["experimental"]; ok {
			return errors.New("only supported with experimental daemon")
		}
	}

	if cmdVersion, ok := cmd.Tags["version"]; ok && versions.LessThan(clientVersion, cmdVersion) {
		return fmt.Errorf("only supported with daemon version >= %s", cmdVersion)
	}

	return nil
}
*/
