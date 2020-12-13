package pirec

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Monitor struct {
	Command  []string
	DevID    string
	ctx      context.Context
	recorder *Recorder
	uploader *Uploader
}

type Config struct {
	Dev              string `json:"dev"`
	Root             string `json:"root"`
	DropboxAuthToken string `json:"dropboxAuthToken"`
	RecordMaxTimeSec int    `json:"recordMaxTimeSec"`
	UploadPeriodSec  int    `json:"uploadPeriodSec"`
}

func NewMonitor(devID string, command []string, rec *Recorder, upl *Uploader) *Monitor {
	return &Monitor{
		Command:  command,
		DevID:    devID,
		ctx:      nil,
		recorder: rec,
		uploader: upl,
	}
}

func CreateMonitor(confFile string) (*Monitor, error) {
	data, err := ioutil.ReadFile(confFile)
	if err != nil {
		return nil, err
	}

	conf := Config{}
	err = json.Unmarshal(data, &conf)
	if err != nil {
		return nil, err
	}

	rec, err := CreateRecorder(conf.Root, time.Duration(conf.RecordMaxTimeSec)*time.Second, conf.Dev)
	if err != nil {
		return nil, err
	}

	upl := NewUploader(conf.Root, time.Duration(conf.UploadPeriodSec)*time.Second, conf.DropboxAuthToken)

	mon := NewMonitor(conf.Dev, []string{"dbus-monitor", "--system"}, rec, upl)

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

	go m.uploader.Start(m.ctx)
	go m.scan(scanner)

	select {
	case <-m.ctx.Done():
		fmt.Println("Monitor: terminating")
		m.uploader.Stop()
		break
	}

	return nil
}
