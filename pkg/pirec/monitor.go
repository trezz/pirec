package pirec

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type Monitor struct {
	Command  []string
	DevID    string
	ctx      context.Context
	recorder *Recorder
}

func NewMonitor(devID string, command []string, rec *Recorder) *Monitor {
	return &Monitor{
		Command:  command,
		DevID:    devID,
		ctx:      nil,
		recorder: rec,
	}
}

func NewDefaultMonitor() (*Monitor, error) {
	devID := "74:45:CE:59:CF:A0"
	rec, err := NewDefaultRecorder(devID)
	if err != nil {
		return nil, err
	}

	mon := NewMonitor(devID, []string{"dbus-monitor", "--system"}, rec)

	return mon, nil
}

func (m *Monitor) devLogPattern() string {
	return fmt.Sprintf("dev_%v", strings.Replace(m.DevID, ":", "_", -1))
}

func (m *Monitor) processSignal(lines []string) error {
	for i := 0; i < len(lines)-1; i += 2 {
		line := lines[i]
		nextLine := lines[i+1]
		if strings.Contains(line, `string "Connected"`) {
			items := strings.Split(nextLine, " ")
			connected, err := strconv.ParseBool(items[len(items)-1])
			if err != nil {
				return err
			}
			fmt.Println("Connected: ", connected)

			if connected {
				go m.recorder.Start(m.ctx)
			} else {
				m.recorder.Stop()
			}

			break
		}
	}

	return nil
}

func (m *Monitor) scan(scanner *bufio.Scanner) error {
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "signal") && strings.Contains(line, m.devLogPattern()) {
			fmt.Println("Monitor: event received")
			// Get full signal message lines.
			msgLines := []string{line}
			for scanner.Scan() {
				nextLine := scanner.Text()
				if strings.HasPrefix(nextLine, "signal") {
					break
				}
				msgLines = append(msgLines, nextLine)
			}
			// Process signal.
			if err := m.processSignal(msgLines); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Monitor) Start(ctx context.Context) error {
	m.ctx = ctx

	cmd := exec.Command(m.Command[0], m.Command[1:]...)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(out)
	if err = cmd.Start(); err != nil {
		return err
	}

	fmt.Println("Monitor: started")

	go m.scan(scanner)

	select {
	case <-m.ctx.Done():
		fmt.Println("Monitor: terminating")
		break
	}

	return nil
}
