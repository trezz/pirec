package pirec

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Uploader struct {
	RootPath         string
	Period           time.Duration
	DropBoxAuthToken string
	AzureAuthToken   string
	cancel           func()
}

func NewUploader(from string, period time.Duration, dropBoxToken, azureToken string) *Uploader {
	return &Uploader{
		RootPath:         from,
		Period:           period,
		DropBoxAuthToken: dropBoxToken,
		AzureAuthToken:   azureToken,
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

	req.Header.Add("Authorization", "Bearer "+u.DropBoxAuthToken)
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

	if len(fileList) <= 1 {
		return nil // Need to work on already compressed data.
	}

	for _, f := range fileList {
		if err = u.upload(f); err != nil {
			return err
		}
		err = exec.Command("gunzip", f).Run()
		if err != nil {
			return fmt.Errorf("gunzip: %v %w", f, err)
		}

		f = strings.TrimSuffix(f, ".gz")

		if !strings.HasSuffix(f, ".raw.wav") {
			continue
		}
		cleanedFile := strings.TrimSuffix(f, ".raw.wav") + ".trim.wav"
		err = TrimSilence(f, cleanedFile)
		if err != nil {
			return fmt.Errorf("trim silence: %w", err)
		} else {
			fmt.Println("Cleaned", cleanedFile)
		}

		text, err := SpeechToText(cleanedFile, "francecentral", u.AzureAuthToken, "fr-FR")
		if err != nil {
			return fmt.Errorf("conversion: %w", err)
		} else {
			fmt.Println("Converted text:", text)
		}

		textFile := cleanedFile + ".txt"
		err = ioutil.WriteFile(textFile, []byte(text), 0644)
		if err != nil {
			return err
		}

		if err = u.upload(textFile); err != nil {
			return err
		}
		if err = os.Remove(textFile); err != nil {
			return err
		}
		if err = os.Remove(cleanedFile); err != nil {
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
