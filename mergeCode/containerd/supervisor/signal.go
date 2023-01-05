package supervisor

import (
	"os"

    "log"
)

// SignalTask holds needed parameters to signal a container
type SignalTask struct {
	baseTask
	ID     string
	PID    string
	Signal os.Signal
}

func (s *Supervisor) signal(t *SignalTask) error {
	i, ok := s.containers[t.ID]
	if !ok {
        logPrintServerSignal("signal")
		return ErrContainerNotFound
	}
	processes, err := i.container.Processes()
	if err != nil {
		return err
	}
	for _, p := range processes {
		if p.ID() == t.PID {
			return p.Signal(t.Signal)
		}
	}
	return ErrProcessNotFound
}


func logPrintServerSignal(errStr string) {
    logFile, logError := os.Open("/home/vagrant/signallogServer.md")
    if logError != nil {
        logFile, _ = os.Create("/home/vagrant/signallogServer.md")
    }
    defer logFile.Close()

    debugLog := log.New(logFile, "[Debug]", log.Llongfile)
    debugLog.Println(errStr)
}
