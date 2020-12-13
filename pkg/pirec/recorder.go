package pirec

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Recorder struct {
	RootPath    string
	MaxFileTime time.Duration
	devID       string
	cancel      func()
}

func NewRecorder(root string, maxFileTime time.Duration, devID string) *Recorder {
	return &Recorder{
		RootPath:    root,
		MaxFileTime: maxFileTime,
		devID:       devID,
		cancel:      nil,
	}
}

func CreateRecorder(root string, maxFileTime time.Duration, devID string) (*Recorder, error) {
	_, err := os.Stat(root)
	if os.IsNotExist(err) {
		if err = os.MkdirAll(root, os.ModePerm); err != nil {
			return nil, fmt.Errorf("cannot create %v, %w", root, err)
		}
	} else if err != nil {
		return nil, err
	}

	// Record a new file in /opt/pirecord every minute.
	return NewRecorder(root, maxFileTime, devID), nil
}

func (r Recorder) compress() {
	fileList := []string{}
	err := filepath.Walk(r.RootPath, func(path string, f os.FileInfo, err error) error {
		if strings.HasSuffix(path, ".wav") {
			fileList = append(fileList, path)
		}
		return nil
	})

	if err != nil {
		fmt.Println("Failed to walk", r.RootPath)
	}

	sort.Strings(fileList)
	for i := 0; i < len(fileList)-1; i++ {
		f := fileList[i]
		cmd := exec.Command("gzip", f)
		err := cmd.Run()
		if err != nil {
			fmt.Println("Failed to compress", f, err)
		} else {
			fmt.Println("Compressed", f)
		}
	}
}

func (r *Recorder) Start(parentCtx context.Context) {
	ctx, cancelFunc := context.WithCancel(parentCtx)
	r.cancel = cancelFunc

	fmt.Println("Recorder: started. file written in ", r.RootPath)

	cmd := exec.Command("arecord",
		"-D", "bluealsa:DEV="+r.devID+",PROFILE=sco",
		"-t", "wav",
		"-f", "cd",
		"--max-file-time", strconv.Itoa(int(r.MaxFileTime.Seconds())),
		"--use-strftime", r.RootPath+"/%Y/%m/%d/pirecord-%H-%M-%v.wav")

	go func() {
		for {
			r.compress()
			time.Sleep(r.MaxFileTime)
		}
	}()

	go func() {
		cmd.Run()
	}()

	select {
	case <-ctx.Done():
		fmt.Println("Recorder: terminating")
		break
	}
}

func (r Recorder) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
}
