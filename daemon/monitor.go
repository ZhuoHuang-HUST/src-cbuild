package daemon

import (
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/libcontainerd"
	"github.com/docker/docker/restartmanager"
)

// StateChanged updates daemon state changes from containerd
func (daemon *Daemon) StateChanged(id string, e libcontainerd.StateInfo) error {
	c := daemon.containers.Get(id)
	if c == nil {
        fmt.Println("daemon/monitor.go   no such container")
		return fmt.Errorf("no such container: %s", id)
	}

    fmt.Println("daemon/monitor.go  StateChanged  id: ", id)
    fmt.Println("daemon/monitor.go  StateChanged  State: ", e.State)
    fmt.Println("daemon/monitor.go  StateChanged  Pid: ", e.Pid)
    fmt.Println("daemon/monitor.go  StateChanged  ProcessID: ", e.ProcessID)

	switch e.State {
	case libcontainerd.StateOOM:
		// StateOOM is Linux specific and should never be hit on Windows
		if runtime.GOOS == "windows" {
			return errors.New("Received StateOOM from libcontainerd on Windows. This should never happen.")
		}
		daemon.updateHealthMonitor(c)
		daemon.LogContainerEvent(c, "oom")
	case libcontainerd.StateExit:
		// if container's AutoRemove flag is set, remove it after clean up
		autoRemove := func() {
			if c.HostConfig.AutoRemove {
				if err := daemon.ContainerRm(c.ID, &types.ContainerRmConfig{ForceRemove: true, RemoveVolume: true}); err != nil {
					logrus.Errorf("can't remove container %s: %v", c.ID, err)
				}
			}
		}

		c.Lock()
		c.StreamConfig.Wait()
		c.Reset(false)

		restart, wait, err := c.RestartManager().ShouldRestart(e.ExitCode, false, time.Since(c.StartedAt))
		if err == nil && restart {
			c.RestartCount++
			c.SetRestarting(platformConstructExitStatus(e))
		} else {
			c.SetStopped(platformConstructExitStatus(e))
			defer autoRemove()
		}

		daemon.updateHealthMonitor(c)
		attributes := map[string]string{
			"exitCode": strconv.Itoa(int(e.ExitCode)),
		}
		daemon.LogContainerEventWithAttributes(c, "die", attributes)
		daemon.Cleanup(c)

		if err == nil && restart {
			go func() {
				err := <-wait
				if err == nil {
					if err = daemon.containerStart(c, "", "", false); err != nil {
						logrus.Debugf("failed to restart container: %+v", err)
					}
				}
				if err != nil {
					c.SetStopped(platformConstructExitStatus(e))
					defer autoRemove()
					if err != restartmanager.ErrRestartCanceled {
						logrus.Errorf("restartmanger wait error: %+v", err)
					}
				}
			}()
		}

		defer c.Unlock()
		if err := c.ToDisk(); err != nil {
			return err
		}
		return daemon.postRunProcessing(c, e)
	case libcontainerd.StateExitProcess:

        fmt.Println("daemon/monitor.go  StateExitProcess")
		if execConfig := c.ExecCommands.Get(e.ProcessID); execConfig != nil {
			ec := int(e.ExitCode)
			execConfig.Lock()
			defer execConfig.Unlock()
			execConfig.ExitCode = &ec
			execConfig.Running = false
			execConfig.StreamConfig.Wait()
			if err := execConfig.CloseStreams(); err != nil {
				logrus.Errorf("%s: %s", c.ID, err)
			}

			// remove the exec command from the container's store only and not the
			// daemon's store so that the exec command can be inspected.
			c.ExecCommands.Delete(execConfig.ID)
		} else {
			logrus.Warnf("Ignoring StateExitProcess for %v but no exec command found", e)
		}
	case libcontainerd.StateStart, libcontainerd.StateRestore:

        fmt.Println("daemon/monitor.go  case StateStart")
		// Container is already locked in this case
		c.SetRunning(int(e.Pid), e.State == libcontainerd.StateStart)
        fmt.Println("daemon/monitor.go case StateStart after Running running : ", int(e.Pid))
		c.HasBeenManuallyStopped = false
		c.HasBeenStartedBefore = true
		if err := c.ToDisk(); err != nil {
            fmt.Println("daemon/monitor.go case StateStart toDisk  err")
			c.Reset(false)
			return err
		}
        fmt.Println("daemon/monitor.go case StateStart before iHealth")
		daemon.initHealthMonitor(c)
        fmt.Println("daemon/monitor.go case StateStart initHealthMonitor")
		daemon.LogContainerEvent(c, "start")
        fmt.Println("daemon/monitor.go case StateStart LogContainerEvent")
        fmt.Println("daemon/monitor.go case StateStart sleep 10 seconds")
        time.Sleep(time.Second * 10)
	case libcontainerd.StatePause:
		// Container is already locked in this case
		c.Paused = true
		if err := c.ToDisk(); err != nil {
			return err
		}
		daemon.updateHealthMonitor(c)
		daemon.LogContainerEvent(c, "pause")
	case libcontainerd.StateResume:
		// Container is already locked in this case
		c.Paused = false
		if err := c.ToDisk(); err != nil {
			return err
		}
		daemon.updateHealthMonitor(c)
		daemon.LogContainerEvent(c, "unpause")
	}

	return nil
}


func (daemon *Daemon) GetFirstContainerBuildingStatus(id string) bool {
      fmt.Println("daemon/monitor.go GetFirstContainerBuildingStatus()")
      container, err := daemon.GetContainer(id)
      if err != nil {
         fmt.Println("daemon/monitor.go GetFirstContainerBuildingStatus() error!!!")
         return false
      }

      return container.GetBuildingStatus()
}


func (daemon *Daemon) TriggerExitEvent(cId string) error {
     fmt.Println("daemon/monitor.go TriggerExitEvent()")

     if err :=  daemon.containerd.TriggerHandleStream(cId); err != nil {
        fmt.Println("daemon/monitor.go TriggerExitEvent() error!!!")
        return err
     }
     
     return nil
}
