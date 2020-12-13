package pirec

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Uploader struct {
	RootPath  string
	Period    time.Duration
	AuthToken string
	cancel    func()
}

func NewUploader(from string, period time.Duration, token string) *Uploader {
	return &Uploader{
		RootPath:  from,
		Period:    period,
		AuthToken: token,
	}
}

func (u Uploader) upload(file string) error {
	dstPath := strings.TrimPrefix(file, u.RootPath)

	fmt.Println("Uploader: uploading", file, "to", dstPath)

	reader, err := os.Open(file)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://content.dropboxapi.com/2/files/upload", reader)
	if err != nil {
		return err
	}

	args := struct {
		Path           string `json:"path"`
		Mode           string `json:"mode"`
		Autorename     bool   `json:"autorename"`
		Mute           bool   `json:"mute"`
		StrictConflict bool   `json:"strict_conflict"`
	}{
		Path:           dstPath,
		Mode:           "add",
		Autorename:     true,
		Mute:           false,
		StrictConflict: false,
	}
	data, err := json.Marshal(args)
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", "Bearer "+u.AuthToken)
	req.Header.Add("Dropbox-API-Arg", string(data))
	req.Header.Add("Content-Type", "application/octet-stream")

	res, err := http.DefaultClient.Do(req)

	if err != nil {
		return err
	}

	defer res.Body.Close()

	if res.StatusCode != 200 {
		return fmt.Errorf("%v", res)
	}

	return nil
}

func (u Uploader) uploadAndClean() error {
	fileList := []string{}
	err := filepath.Walk(u.RootPath, func(path string, f os.FileInfo, err error) error {
		if strings.HasSuffix(path, ".gz") {
			fileList = append(fileList, path)
		}
		return nil
	})

	if err != nil {
		return err
	}

	sort.Strings(fileList)
	for _, f := range fileList {
		if err = u.upload(f); err != nil {
			return err
		}
		if err = os.Remove(f); err != nil {
			return err
		}
	}

	return nil
}

func (u *Uploader) Start(parentCtx context.Context) {
	ctx, cancelFunc := context.WithCancel(parentCtx)
	u.cancel = cancelFunc

	fmt.Println("Uploader: started")

	go func() {
		for {
			err := u.uploadAndClean()
			if err != nil {
				fmt.Println("Uploader:", err)
			}
			time.Sleep(u.Period)
		}
	}()

	select {
	case <-ctx.Done():
		fmt.Println("Uploader: terminating")
		break
	}
}

func (u Uploader) Stop() {
	if u.cancel != nil {
		u.cancel()
	}
}
