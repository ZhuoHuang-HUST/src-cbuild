package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"strings"
	"time"

    "golang.org/x/net/context"

    "github.com/docker/docker/api/types"
//    "github.com/docker/docker/utils"
//    "github.com/docker/docker/pkg/term"
//    "github.com/docker/docker/api/types/strslice"
//    "github.com/docker/docker/daemon/exec"
//    "github.com/docker/docker/pkg/pools"
//    "github.com/docker/docker/pkg/signal"


//    "github.com/Sirupsen/logrus"
//    "github.com/docker/docker/api/errors"
//    "github.com/docker/docker/libcontainerd"


	"github.com/docker/docker/api/types/backend"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/builder/dockerfile"
	"github.com/docker/docker/container"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/reference"
)


// merge merges two Config, the image container configuration (defaults values),
// and the user container configuration, either passed by the API or generated
// by the cli.
// It will mutate the specified user configuration (userConf) with the image
// configuration where the user configuration is incomplete.
func merge(userConf, imageConf *containertypes.Config) error {
	if userConf.User == "" {
		userConf.User = imageConf.User
	}
	if len(userConf.ExposedPorts) == 0 {
		userConf.ExposedPorts = imageConf.ExposedPorts
	} else if imageConf.ExposedPorts != nil {
		for port := range imageConf.ExposedPorts {
			if _, exists := userConf.ExposedPorts[port]; !exists {
				userConf.ExposedPorts[port] = struct{}{}
			}
		}
	}

	if len(userConf.Env) == 0 {
		userConf.Env = imageConf.Env
	} else {
		for _, imageEnv := range imageConf.Env {
			found := false
			imageEnvKey := strings.Split(imageEnv, "=")[0]
			for _, userEnv := range userConf.Env {
				userEnvKey := strings.Split(userEnv, "=")[0]
				if runtime.GOOS == "windows" {
					// Case insensitive environment variables on Windows
					imageEnvKey = strings.ToUpper(imageEnvKey)
					userEnvKey = strings.ToUpper(userEnvKey)
				}
				if imageEnvKey == userEnvKey {
					found = true
					break
				}
			}
			if !found {
				userConf.Env = append(userConf.Env, imageEnv)
			}
		}
	}

	if userConf.Labels == nil {
		userConf.Labels = map[string]string{}
	}
	if imageConf.Labels != nil {
		for l := range userConf.Labels {
			imageConf.Labels[l] = userConf.Labels[l]
		}
		userConf.Labels = imageConf.Labels
	}

	if len(userConf.Entrypoint) == 0 {
		if len(userConf.Cmd) == 0 {
			userConf.Cmd = imageConf.Cmd
			userConf.ArgsEscaped = imageConf.ArgsEscaped
		}

		if userConf.Entrypoint == nil {
			userConf.Entrypoint = imageConf.Entrypoint
		}
	}
	if imageConf.Healthcheck != nil {
		if userConf.Healthcheck == nil {
			userConf.Healthcheck = imageConf.Healthcheck
		} else {
			if len(userConf.Healthcheck.Test) == 0 {
				userConf.Healthcheck.Test = imageConf.Healthcheck.Test
			}
			if userConf.Healthcheck.Interval == 0 {
				userConf.Healthcheck.Interval = imageConf.Healthcheck.Interval
			}
			if userConf.Healthcheck.Timeout == 0 {
				userConf.Healthcheck.Timeout = imageConf.Healthcheck.Timeout
			}
			if userConf.Healthcheck.Retries == 0 {
				userConf.Healthcheck.Retries = imageConf.Healthcheck.Retries
			}
		}
	}

	if userConf.WorkingDir == "" {
		userConf.WorkingDir = imageConf.WorkingDir
	}
	if len(userConf.Volumes) == 0 {
		userConf.Volumes = imageConf.Volumes
	} else {
		for k, v := range imageConf.Volumes {
			userConf.Volumes[k] = v
		}
	}

	if userConf.StopSignal == "" {
		userConf.StopSignal = imageConf.StopSignal
	}
	return nil
}

// Commit creates a new filesystem image from the current state of a container.
// The image can optionally be tagged into a repository.
func (daemon *Daemon) Commit(name string, c *backend.ContainerCommitConfig) (string, error) {
	start := time.Now()

    fmt.Println("daemon/commit.go  Commit()")

	container, err := daemon.GetContainer(name)
	if err != nil {
		return "", err
	}
    fmt.Println("daemon/commit.go  Commit() container.ImageID : ", container.ImageID)
    fmt.Println("daemon/commit.go  Commit() name : ", name)

	// It is not possible to commit a running container on Windows and on Solaris.
	if (runtime.GOOS == "windows" || runtime.GOOS == "solaris") && container.IsRunning() {
		return "", fmt.Errorf("%+v does not support commit of a running container", runtime.GOOS)
	}


    tmpConfig := container.Config
    fmt.Println("daemon/commit.go  judge the status c.Pause container container.IsPause()",c.Pause , container.IsPaused())
    fmt.Println("daemon/commit.go  container Config ", tmpConfig)

/*	if c.Pause && !container.IsPaused() {
		daemon.containerPause(container)
		defer daemon.containerUnpause(container)
	}
*/
    fmt.Println("daemon/commit.go  not Paused!!!!!!!!!!!!!!")


	newConfig, err := dockerfile.BuildFromConfig(c.Config, c.Changes)
	if err != nil {
		return "", err
	}


    fmt.Println("daemon/commit.go   merge config")
	if c.MergeConfigs {
		if err := merge(newConfig, container.Config); err != nil {
			return "", err
		}
	}

    fmt.Println("daemon/commit.go  before exportContainerRw container : ", container)
	rwTar, err := daemon.exportContainerRw(container)
	if err != nil {
        fmt.Println("daemon/commit.go  exportContainerRw is err!!!")
		return "", err
	}
	defer func() {
		if rwTar != nil {
			rwTar.Close()
		}
	}()

	var history []image.History
	rootFS := image.NewRootFS()
	osVersion := ""
	var osFeatures []string

	if container.ImageID != "" {
        fmt.Println("daemon/commit.go  container.ImageID : ", container.ImageID)
		img, err := daemon.imageStore.Get(container.ImageID)
		if err != nil {
			return "", err
		}
		history = img.History
		rootFS = img.RootFS
		osVersion = img.OSVersion
		osFeatures = img.OSFeatures
	}

    fmt.Println("daemon/commit.go  before register()")
	l, err := daemon.layerStore.Register(rwTar, rootFS.ChainID())
	if err != nil {
		return "", err
	}
    fmt.Println("daemon/commit.go  after register()")
	defer layer.ReleaseAndLog(daemon.layerStore, l)

	h := image.History{
		Author:     c.Author,
		Created:    time.Now().UTC(),
		CreatedBy:  strings.Join(container.Config.Cmd, " "),
		Comment:    c.Comment,
		EmptyLayer: true,
	}

    fmt.Println("daemon/commit.go  before diff()")
	if diffID := l.DiffID(); layer.DigestSHA256EmptyTar != diffID {
		h.EmptyLayer = false
		rootFS.Append(diffID)
	}

	history = append(history, h)

	config, err := json.Marshal(&image.Image{
		V1Image: image.V1Image{
			DockerVersion:   dockerversion.Version,
			Config:          newConfig,
			Architecture:    runtime.GOARCH,
			OS:              runtime.GOOS,
			Container:       container.ID,
			ContainerConfig: *container.Config,
			Author:          c.Author,
			Created:         h.Created,
		},
		RootFS:     rootFS,
		History:    history,
		OSFeatures: osFeatures,
		OSVersion:  osVersion,
	})

	if err != nil {
		return "", err
	}

    fmt.Println("daemon/commit.go  before create()")
	id, err := daemon.imageStore.Create(config)
    fmt.Println("daemon/commit.go Commit finish creat image")

	if err != nil {
		return "", err
	}

	if container.ImageID != "" {
		if err := daemon.imageStore.SetParent(id, container.ImageID); err != nil {
			return "", err
		}
	}

	imageRef := ""
	if c.Repo != "" {
		newTag, err := reference.WithName(c.Repo) // todo: should move this to API layer
		if err != nil {
			return "", err
		}
		if c.Tag != "" {
			if newTag, err = reference.WithTag(newTag, c.Tag); err != nil {
				return "", err
			}
		}
		if err := daemon.TagImageWithReference(id, newTag); err != nil {
			return "", err
		}
		imageRef = newTag.String()
	}

	attributes := map[string]string{
		"comment":  c.Comment,
		"imageID":  id.String(),
		"imageRef": imageRef,
	}
	daemon.LogContainerEventWithAttributes(container, "commit", attributes)
	containerActions.WithValues("commit").UpdateSince(start)
	return id.String(), nil
}


func (daemon *Daemon) GetFirstContainerStatus(id string) error {
      container, err := daemon.GetContainer(id)
      if err != nil {
         return err
      }
      fmt.Println("daemon/commit.go GetFirstContainerStatus() isRunning ", container.IsRunning())
      return nil
}

/*
//func (daemon *Daemon) GetFirstContainer(id string) (*container.Container, error) {
      container, err := daemon.GetContainer(id)
      if err != nil {
         return "", err
      }
      fmt.Println("daemon/commit.go GetFirstContainer()  return first container")
    
      return container, err
}
*/
/*
//func (daemon *Daemon) GetFirstContainerBuildingStatus(id string) error {
      container, err := daemon.GetContainer(id)
      if err != nil {
         return err
      }
      fmt.Println("daemon/commit.go GetFirstContainerBuildingStatus() isBuilding : ", container.GetBuildingStatus())

      return nil
}
*/

func (daemon *Daemon) SetFirstContainerBuildingStatus(cId string, status bool) error {
      container, err := daemon.GetContainer(cId)
      if err != nil {
         return err
      }

      fmt.Println("daemon/commit.go SetFirstContainerBuildingStatus() isBuilding before : ", container.GetBuildingStatus())
      container.SetBuildingStatus(status)
      fmt.Println("daemon/commit.go SetFirstContainerBuildingStatus() isBuilding after : ", container.GetBuildingStatus())
      
      return nil
}


// ContainerExecCreate sets up an exec in a running container.
func (d *Daemon) FirstContainerExecCreate(name string, config *types.ExecConfig) (string, error) {

    fmt.Println("daemon/commit.go  FirstContainerExecCreate()")

    id, err := d.ContainerExecCreate(name, config)
    if err != nil {
       fmt.Println("daemon/commit.go  ContainerExecCreate() is err!!!")
       return "", err
    }

    fmt.Println("daemon/commit.go  FirstContainerExecCreate() end")
    return id, err


/*    container, err := d.getActiveContainer(name)
	if err != nil {
		return "", err
	}

	cmd := strslice.StrSlice(config.Cmd)
	entrypoint, args := d.getEntrypointAndArgs(strslice.StrSlice{}, cmd)

	keys := []byte{}
	if config.DetachKeys != "" {
		keys, err = term.ToBytes(config.DetachKeys)
		if err != nil {
			err = fmt.Errorf("Invalid escape keys (%s) provided", config.DetachKeys)
			return "", err
		}
	}

	execConfig := exec.NewConfig()
	execConfig.OpenStdin = config.AttachStdin
	execConfig.OpenStdout = config.AttachStdout
	execConfig.OpenStderr = config.AttachStderr
	execConfig.ContainerID = container.ID
	execConfig.DetachKeys = keys
	execConfig.Entrypoint = entrypoint
	execConfig.Args = args
	execConfig.Tty = config.Tty
	execConfig.Privileged = config.Privileged
	execConfig.User = config.User

	linkedEnv, err := d.setupLinkedContainers(container)
	if err != nil {
		return "", err
	}
	execConfig.Env = utils.ReplaceOrAppendEnvValues(container.CreateDaemonEnvironment(config.Tty, linkedEnv), config.Env)
	if len(execConfig.User) == 0 {
		execConfig.User = container.Config.User
	}

	d.registerExecCommand(container, execConfig)

	d.LogContainerEvent(container, "exec_create: "+execConfig.Entrypoint+" "+strings.Join(execConfig.Args, " "))

	return execConfig.ID, nil
    */
}



// ContainerExecStart starts a previously set up exec instance. The
// std streams are set up.
// If ctx is cancelled, the process is terminated.
func (d *Daemon) FirstContainerExecStart(ctx context.Context, name string, stdin io.ReadCloser, stdout io.Writer, stderr io.Writer) (err error) {

    fmt.Println("daemon/commit.go  FirstContainerExecStart()")

    if err := d.ContainerExecStart(ctx, name, stdin, stdout, stderr); err != nil {
         fmt.Println("daemon/commit.go  FirstContainerExecStart() is err : ", err)
    }

/*
	var (
		cStdin           io.ReadCloser
		cStdout, cStderr io.Writer
	)

	ec, err := d.getExecConfig(name)
	if err != nil {
		return errExecNotFound(name)
	}
    fmt.Println("daemon/commit.go  FirstContainerExecStart() execConfig : ", ec)

	ec.Lock()
	if ec.ExitCode != nil {
		ec.Unlock()
		err := fmt.Errorf("Error: Exec command %s has already run", ec.ID)
		return errors.NewRequestConflictError(err)
	}

	if ec.Running {
		ec.Unlock()
		return fmt.Errorf("Error: Exec command %s is already running", ec.ID)
	}
	ec.Running = true
	defer func() {
		if err != nil {
			ec.Running = false
			exitCode := 126
			ec.ExitCode = &exitCode
		}
	}()
	ec.Unlock()

	c := d.containers.Get(ec.ContainerID)
	fmt.Println("daemon/commit.go  FirstContainerExecStart  starting exec command : ", ec.ID)
    fmt.Println("daemon/commit.go  FirstContainerExecStart  in container : ", c.ID)

	d.LogContainerEvent(c, "exec_start: "+ec.Entrypoint+" "+strings.Join(ec.Args, " "))

	if ec.OpenStdin && stdin != nil {
		r, w := io.Pipe()
		go func() {
			defer w.Close()
			defer logrus.Debug("Closing buffered stdin pipe")
			pools.Copy(w, stdin)
		}()
		cStdin = r
	}
	if ec.OpenStdout {
		cStdout = stdout
	}
	if ec.OpenStderr {
		cStderr = stderr
	}

	if ec.OpenStdin {
		ec.StreamConfig.NewInputPipes()
	} else {
		ec.StreamConfig.NewNopInputPipe()
	}

	p := libcontainerd.Process{
		Args:     append([]string{ec.Entrypoint}, ec.Args...),
		Env:      ec.Env,
		Terminal: ec.Tty,
	}

	if err := execSetPlatformOpt(c, ec, &p); err != nil {
		return err
	}

	attachErr := container.AttachStreams(ctx, ec.StreamConfig, ec.OpenStdin, true, ec.Tty, cStdin, cStdout, cStderr, ec.DetachKeys)

    fmt.Println("daemon/commit.go  FirstContainerExecStart()  AddProcess()")
	systemPid, err := d.containerd.AddProcess(ctx, c.ID, name, p, ec.InitializeStdio)
	if err != nil {
        fmt.Println("daemon/commit.go  FirstContainerExecStart()  AddProcess() err!!!")
		return err
	}
    fmt.Println("daemon/commit.go  FirstContainerExecStart()  AddProcess systemPid : ", systemPid)

	ec.Lock()
	ec.Pid = systemPid
	ec.Unlock()

	select {
	case <-ctx.Done():
		logrus.Debugf("Sending TERM signal to process %v in container %v", name, c.ID)
        fmt.Println("daemon/commit.go FirstContainerExecStart() sendingterm signal")
		d.containerd.SignalProcess(c.ID, name, int(signal.SignalMap["TERM"]))
		select {
		case <-time.After(termProcessTimeout * time.Second):
			logrus.Infof("Container %v, process %v failed to exit within %d seconds of signal TERM - using the force", c.ID, name, termProcessTimeout)
            fmt.Println("daemon/commit.go FirstContainerExecStart() failed to exit termProcessTimeout ", termProcessTimeout)
			d.containerd.SignalProcess(c.ID, name, int(signal.SignalMap["KILL"]))
		case <-attachErr:
			// TERM signal worked
            fmt.Println("daemon/commit.go FirstExecContainer() TERM signal worked")
		}
		return fmt.Errorf("context cancelled")
	case err := <-attachErr:
        fmt.Println("daemon/commit.go FirstContainerExecStart() attachErr")
		if err != nil {
			if _, ok := err.(container.DetachError); !ok {
				return fmt.Errorf("exec attach failed with error: %v", err)
			}
			d.LogContainerEvent(c, "exec_detach")
		}
	}
  */  
    fmt.Println("daemon/commit.go FirstContainerExecStart() end")
	return nil
}



// It will also return the error produced by `getConfig`
func (d *Daemon) FirstContainerExecExists(name string) (bool, error) {
	if _, err := d.getExecConfig(name); err != nil {
		return false, err
	}
	return true, nil
}




func (daemon *Daemon) exportContainerRw(container *container.Container) (io.ReadCloser, error) {
	fmt.Println("daemon/commit.go exportContainerRw")
    
    if err := daemon.Mount(container); err != nil {
		return nil, err
	}

	archive, err := container.RWLayer.TarStream()
	if err != nil {
		daemon.Unmount(container) // logging is already handled in the `Unmount` function
		return nil, err
	}
	return ioutils.NewReadCloserWrapper(archive, func() error {
			archive.Close()
			return container.RWLayer.Unmount()
		}),
		nil
}
